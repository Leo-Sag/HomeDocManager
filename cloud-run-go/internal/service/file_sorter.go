package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/leo-sagawa/homedocmanager/internal/config"
	"github.com/leo-sagawa/homedocmanager/internal/model"
)

// FileSorter はファイル仕分けサービス
type FileSorter struct {
	aiRouter       *AIRouter
	pdfProcessor   *PDFProcessor
	driveClient    *DriveClient
	photosClient   *PhotosClient
	calendarClient *CalendarClient
	tasksClient    *TasksClient
	notebooklmSync *NotebookLMSync
	gradeManager   *GradeManager

	// 並行処理制御
	processingMu    sync.Mutex
	processingFiles map[string]bool
}

// NewFileSorter は新しいFileSorterを作成
func NewFileSorter(
	aiRouter *AIRouter,
	pdfProcessor *PDFProcessor,
	driveClient *DriveClient,
	photosClient *PhotosClient,
	calendarClient *CalendarClient,
	tasksClient *TasksClient,
	notebooklmSync *NotebookLMSync,
	gradeManager *GradeManager,
) *FileSorter {
	return &FileSorter{
		aiRouter:        aiRouter,
		pdfProcessor:    pdfProcessor,
		driveClient:     driveClient,
		photosClient:    photosClient,
		calendarClient:  calendarClient,
		tasksClient:     tasksClient,
		notebooklmSync:  notebooklmSync,
		gradeManager:    gradeManager,
		processingFiles: make(map[string]bool),
	}
}

// ProcessFile はファイルを処理
func (fs *FileSorter) ProcessFile(ctx context.Context, fileID string) model.ProcessResult {
	// インメモリロックによる並行処理防止（最優先）
	fs.processingMu.Lock()
	if fs.processingFiles[fileID] {
		fs.processingMu.Unlock()
		log.Printf("別のリクエストで処理中のためスキップ: %s", fileID)
		return model.ProcessResultSkipped
	}
	fs.processingFiles[fileID] = true
	fs.processingMu.Unlock()

	// 処理完了時にロックを解放
	defer func() {
		fs.processingMu.Lock()
		delete(fs.processingFiles, fileID)
		fs.processingMu.Unlock()
	}()

	// ファイル情報を取得
	fileInfo, err := fs.driveClient.GetFile(ctx, fileID)
	if err != nil {
		log.Printf("ファイル情報取得失敗: %v", err)
		return model.ProcessResultError
	}

	// Inbox（SOURCE）フォルダ以外のファイルは仕分け（移動・リネーム）対象外とする
	sourceID := config.FolderIDs["SOURCE"]
	inInbox := false
	for _, parentID := range fileInfo.Parents {
		if parentID == sourceID {
			inInbox = true
			break
		}
	}

	if !inInbox {
		log.Printf("Inbox以外のフォルダにあるためスキップします: %s (Parents: %v)", fileInfo.Name, fileInfo.Parents)
		return model.ProcessResultSkipped
	}

	log.Printf("処理開始: %s", fileInfo.Name)

	// 処理済みファイルのチェック（並行処理での重複防止）
	if fs.driveClient.IsFileProcessed(ctx, fileID) {
		log.Printf("既に処理済みのファイルです: %s", fileInfo.Name)
		return model.ProcessResultSkipped
	}

	// 即座に処理中マーカーを設定（並行通知からの重複防止）
	if err := fs.driveClient.MarkFileAsProcessed(ctx, fileID); err != nil {
		log.Printf("Warning: 処理中マーカー設定失敗: %v", err)
		// エラーでも続行（既に他のプロセスが処理中の可能性）
	}

	// 対応ファイル形式をチェック
	if !fs.isSupportedMimeType(fileInfo.MimeType) {
		log.Printf("非対応のファイル形式のためスキップ: %s", fileInfo.MimeType)
		return model.ProcessResultSkipped
	}

	// ファイルをダウンロード
	fileBytes, err := fs.driveClient.DownloadFile(ctx, fileID)
	if err != nil {
		log.Printf("ファイルダウンロード失敗: %v", err)
		return model.ProcessResultError
	}

	// Geminiで解析 (PDFもそのまま渡す)
	var (
		analysisResult *model.AnalysisResult
		combined       *model.DocumentBundle
	)
	if config.EnableCombinedGemini {
		prompt := fs.createAnalysisPrompt(fileInfo.Name)
		bundle, combinedErr := fs.aiRouter.AnalyzeDocumentFull(ctx, fileBytes, fileInfo.MimeType, fileInfo.Name, prompt)
		if combinedErr != nil || bundle == nil || bundle.Analysis == nil {
			log.Printf("統合解析失敗、フォールバックします: %v", combinedErr)
		} else {
			combined = bundle
			analysisResult = bundle.Analysis
		}
	}

	if analysisResult == nil {
		analysisResult, err = fs.analyzeDocument(ctx, fileBytes, fileInfo.MimeType, fileInfo.Name)
		if err != nil {
			log.Printf("Gemini解析失敗: %v", err)
			return model.ProcessResultError
		}
	}

	log.Printf("解析結果: category=%s, child=%s, date=%s, summary=%s",
		analysisResult.Category, analysisResult.ChildName, analysisResult.Date, analysisResult.Summary)

	// 子供の特定とフォルダ解決ロジック
	if analysisResult.Category == "40_子供・教育" {
		fs.processChildEducation(analysisResult)
	}

	// 移動先フォルダを決定
	destinationFolderID, err := fs.getDestinationFolder(ctx, analysisResult)
	if err != nil {
		log.Printf("移動先フォルダ決定失敗: %v", err)
		return model.ProcessResultError
	}

	// 新しいファイル名を生成
	newFileName := fs.generateNewFilename(analysisResult, fileInfo.Name)

	// ファイルをリネーム
	if err := fs.driveClient.RenameFile(ctx, fileID, newFileName); err != nil {
		log.Printf("ファイルリネーム失敗: %v", err)
		return model.ProcessResultError
	}

	// ファイルを移動
	if err := fs.driveClient.MoveFile(ctx, fileID, destinationFolderID); err != nil {
		log.Printf("ファイル移動失敗: %v", err)
		return model.ProcessResultError
	}

	log.Printf("処理完了: %s → %s", fileInfo.Name, newFileName)

	// 追加アクション
	fs.performAdditionalActions(ctx, fileBytes, fileInfo.MimeType, newFileName, fileID, analysisResult, combined)

	return model.ProcessResultProcessed
}

// processChildEducation は子供・教育カテゴリの特殊処理
func (fs *FileSorter) processChildEducation(result *model.AnalysisResult) {
	// 年度計算
	fiscalYear := fs.gradeManager.CalculateFiscalYear(result.Date)
	result.FiscalYear = fiscalYear

	// 子供特定
	var targetChildren []string
	if result.ChildName != "" {
		targetChildren = []string{result.ChildName}
	} else if result.TargetGradeClass != "" {
		targetChildren = fs.gradeManager.IdentifyChildren(result.TargetGradeClass, fiscalYear)
	}

	result.TargetChildren = targetChildren

	// 卒業チェック
	if len(targetChildren) > 0 {
		firstChild := targetChildren[0]
		if fs.gradeManager.IsGraduated(firstChild, fiscalYear) {
			log.Printf("%sは高校卒業後です。大人カテゴリに振り分けます", firstChild)
			result.Category = "30_ライフ・行政"
			result.TargetAdult = firstChild
			result.TargetChildren = []string{}
			result.ChildName = ""
		}
	}

	// フォルダ名の決定
	if len(result.TargetChildren) > 0 {
		folderName, label, emoji := fs.gradeManager.ResolveFolderName(result.TargetChildren)
		if folderName != "" {
			result.ResolvedFolderName = folderName
			result.ResolvedLabel = label
			result.ResolvedEmoji = emoji
		}
	}
}

// getDestinationFolder は移動先フォルダIDを取得
func (fs *FileSorter) getDestinationFolder(ctx context.Context, result *model.AnalysisResult) (string, error) {
	category := result.Category

	// 写真・その他
	if result.IsPhoto || category == "50_写真・その他" {
		return config.FolderIDs["PHOTO_OTHER"], nil
	}

	// 子供・教育
	if category == "40_子供・教育" {
		return fs.getChildrenEduFolder(ctx, result)
	}

	// 年度サブフォルダ対象カテゴリ
	for _, c := range config.CategoriesWithYearSubfolder {
		if category == c {
			return fs.getFolderWithYearSubfolder(ctx, category, result)
		}
	}

	// デフォルト
	folderID, exists := config.CategoryMap[category]
	if !exists {
		return config.FolderIDs["PHOTO_OTHER"], nil
	}
	return folderID, nil
}

// getFolderWithYearSubfolder は年度サブフォルダを取得
func (fs *FileSorter) getFolderWithYearSubfolder(ctx context.Context, category string, result *model.AnalysisResult) (string, error) {
	baseFolderID := config.CategoryMap[category]

	fiscalYear := result.FiscalYear
	if fiscalYear == 0 {
		fiscalYear = fs.gradeManager.CalculateFiscalYear(result.Date)
	}

	yearFolderID, err := fs.driveClient.GetOrCreateFolder(
		ctx,
		fmt.Sprintf("%d年度", fiscalYear),
		baseFolderID,
	)
	if err != nil {
		return "", err
	}

	return yearFolderID, nil
}

// getChildrenEduFolder は子供・教育用のフォルダを取得
func (fs *FileSorter) getChildrenEduFolder(ctx context.Context, result *model.AnalysisResult) (string, error) {
	baseFolderID := config.FolderIDs["CHILDREN_EDU"]

	// フォルダ名を決定
	folderName := result.ResolvedFolderName
	if folderName == "" {
		folderName = result.ChildName
	}
	if folderName == "" {
		folderName = "共通・学校全般"
	}

	// 子供名フォルダ
	childFolderID, err := fs.driveClient.GetOrCreateFolder(ctx, folderName, baseFolderID)
	if err != nil {
		return "", err
	}

	// 年度フォルダ
	fiscalYear := result.FiscalYear
	if fiscalYear == 0 {
		fiscalYear = fs.gradeManager.CalculateFiscalYear(result.Date)
	}

	yearFolderID, err := fs.driveClient.GetOrCreateFolder(
		ctx,
		fmt.Sprintf("%d年度", fiscalYear),
		childFolderID,
	)
	if err != nil {
		return "", err
	}

	// サブカテゴリフォルダ
	subCategory := result.SubCategory
	if subCategory == "" {
		subCategory = "01_お便り・スケジュール"
	}

	subCategoryFolderID, err := fs.driveClient.GetOrCreateFolder(ctx, subCategory, yearFolderID)
	if err != nil {
		return "", err
	}

	return subCategoryFolderID, nil
}

// generateNewFilename は新しいファイル名を生成
func (fs *FileSorter) generateNewFilename(result *model.AnalysisResult, originalName string) string {
	date := result.Date
	if date == "" {
		date = time.Now().Format("20060102")
	}

	summary := result.Summary
	if summary == "" {
		summary = "document"
	}

	// 拡張子を取得
	parts := strings.Split(originalName, ".")
	extension := "pdf"
	if len(parts) > 1 {
		extension = parts[len(parts)-1]
	}

	return fmt.Sprintf("%s_%s.%s", date, summary, extension)
}

// performAdditionalActions は追加アクション（Photos, Calendar, NotebookLM）を実行
func (fs *FileSorter) performAdditionalActions(
	ctx context.Context,
	data []byte,
	mimeType string,
	fileName string,
	fileID string,
	result *model.AnalysisResult,
	combined *model.DocumentBundle,
) {
	category := result.Category
	subCategory := result.SubCategory

	// Google Photosアップロード判定
	shouldUploadToPhotos := category == "50_写真・その他" ||
		(category == "40_子供・教育" && subCategory == "03_記録・作品・成績")

	if fs.photosClient != nil && shouldUploadToPhotos {
		description := fmt.Sprintf("【%s】%s_%s", category, result.Date, result.Summary)

		if mimeType == "application/pdf" {
			// PDFを画像に変換してアップロード
			images, err := fs.pdfProcessor.ConvertPDFToImages(data, config.DPI.Photos)
			if err != nil {
				log.Printf("PDF画像変換失敗: %v", err)
			} else {
				for i, imgData := range images {
					pageDesc := fmt.Sprintf("%s (Page %d/%d)", description, i+1, len(images))
					_, err := fs.photosClient.UploadImage(ctx, imgData, pageDesc)
					if err != nil {
						log.Printf("Google Photosアップロード失敗 (Page %d): %v", i+1, err)
					}
				}
			}
		} else {
			// そのままアップロード
			_, err := fs.photosClient.UploadImage(ctx, data, description)
			if err != nil {
				log.Printf("Google Photosアップロード失敗: %v", err)
			}
		}
	}

	// Calendar/Tasks登録判定
	shouldRegisterCalendar := (category == "40_子供・教育" ||
		(contains([]string{"10_マネー・税務", "30_ライフ・行政"}, category) && result.TargetAdult != ""))

	if shouldRegisterCalendar {
		var precomputed *model.EventsAndTasks
		if combined != nil && combined.EventsAndTasks != nil {
			precomputed = combined.EventsAndTasks
		}
		fs.registerCalendarAndTasks(ctx, data, mimeType, fileName, fileID, result, precomputed)
	}

	// NotebookLM同期
	log.Printf("NotebookLM同期チェック: category=%s, subCategory=%s, sync_enabled=%v", category, subCategory, fs.notebooklmSync != nil)
	if fs.notebooklmSync != nil && fs.notebooklmSync.ShouldSync(category, subCategory) {
		// Avoid re-syncing the same file (saves OCR/Gemini cost).
		if fs.notebooklmSync.IsAlreadySynced(ctx, fileID) {
			log.Printf("NotebookLM同期済みのためスキップ: %s (%s)", fileName, fileID)
			return
		}

		log.Printf("NotebookLM同期対象確定: %s", category)
		// Driveカテゴリ → NotebookLMカテゴリに変換
		notebookCategory, exists := config.NotebookLMCategoryMap[category]
		if !exists {
			log.Printf("NotebookLMカテゴリマッピングが見つかりません: %s", category)
		} else {
			log.Printf("NotebookLM変換後カテゴリ: %s", notebookCategory)
			// OCRBundleを取得（Geminiで抽出）
			var bundle *model.OCRBundle
			if combined != nil && combined.OCRBundle != nil {
				bundle = combined.OCRBundle
			}
			var ocrErr error
			if bundle == nil || bundle.OCRText == "" {
				bundle, ocrErr = fs.aiRouter.ExtractOCRBundle(ctx, data, mimeType)
			}
			if ocrErr != nil {
				log.Printf("OCRBundle抽出失敗: %v", ocrErr)
			} else if bundle != nil && bundle.OCRText != "" {
				// 年度を計算
				fiscalYear := result.FiscalYear
				if fiscalYear == 0 {
					fiscalYear = fs.gradeManager.CalculateFiscalYear(result.Date)
				}

				log.Printf("NotebookLM同期実行開始: %s (%d年度_%s)", fileName, fiscalYear, notebookCategory)
				// NotebookLMに同期
				err := fs.notebooklmSync.SyncFile(ctx, fileID, fileName, notebookCategory, bundle.OCRText, bundle.Facts, bundle.Summary, result.Date, fiscalYear)
				if err != nil {
					log.Printf("NotebookLM同期失敗: %v", err)
				} else {
					log.Printf("NotebookLM同期成功ログ: %s", fileName)
				}
			} else {
				log.Printf("OCRテキストが空のため同期をスキップしました")
			}
		}
	} else {
		log.Printf("NotebookLM同期対象外またはサービス未初期化: category=%s", category)
	}
}

// registerCalendarAndTasks はカレンダー・タスクを登録
func (fs *FileSorter) registerCalendarAndTasks(
	ctx context.Context,
	data []byte,
	mimeType string,
	fileName string,
	fileID string,
	analysisResult *model.AnalysisResult,
	precomputed *model.EventsAndTasks,
) {
	if fs.calendarClient == nil && fs.tasksClient == nil {
		return
	}

	log.Println("カレンダー・タスク抽出処理開始...")

	eventsAndTasks := precomputed
	if eventsAndTasks == nil {
		var err error
		eventsAndTasks, err = fs.aiRouter.ExtractEventsAndTasks(ctx, data, mimeType, fileName)
		if err != nil {
			log.Printf("カレンダー・タスク情報抽出失敗: %v", err)
			return
		}
	}

	// ファイルURL
	fileURL := fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID)

	// プレフィックス作成
	titlePrefix := fs.createTitlePrefix(analysisResult)

	// イベント登録
	if fs.calendarClient != nil {
		for _, event := range eventsAndTasks.Events {
			eventTitle := titlePrefix + " " + event.Title
			// 重複チェック
			exists, err := fs.calendarClient.EventExists(ctx, eventTitle, event.Date)
			if err != nil {
				log.Printf("カレンダー重複チェック失敗: %v", err)
			} else if exists {
				log.Printf("カレンダーイベントは既に存在します: %s", eventTitle)
				continue
			}

			event.Title = eventTitle
			notes := fmt.Sprintf("📎 元のお便り: %s", fileURL)
			_, err = fs.calendarClient.CreateEvent(ctx, &event, notes)
			if err != nil {
				log.Printf("イベント作成失敗: %v", err)
			}
		}
	}

	// タスク登録 (期日が同じものをマージ)
	if fs.tasksClient != nil {
		mergedTasks := make(map[string]*model.Task)
		var dueDates []string // 順序維持のため

		for _, task := range eventsAndTasks.Tasks {
			if existing, ok := mergedTasks[task.DueDate]; ok {
				existing.Title += " / " + task.Title
				if task.Notes != "" {
					if existing.Notes != "" {
						existing.Notes += "\n"
					}
					existing.Notes += task.Notes
				}
			} else {
				t := task
				mergedTasks[task.DueDate] = &t
				dueDates = append(dueDates, task.DueDate)
			}
		}

		for _, dueDate := range dueDates {
			task := mergedTasks[dueDate]
			taskTitle := titlePrefix + " " + task.Title
			// タイトル+期日での重複チェック
			exists, err := fs.tasksClient.TaskExistsByTitleAndDate(ctx, taskTitle, task.DueDate)
			if err != nil {
				log.Printf("タスク重複チェック失敗: %v", err)
			} else if exists {
				log.Printf("タスクは既に存在します: %s (期日: %s)", taskTitle, task.DueDate)
				continue
			}

			notes := fmt.Sprintf("📎 元のお便り: %s", fileURL)
			if task.Notes != "" {
				notes += "\n\n" + task.Notes
			}
			_, err = fs.tasksClient.CreateTask(ctx, task, notes)
			if err != nil {
				log.Printf("タスク作成失敗: %v", err)
			}
		}
	}
}

// createTitlePrefix はタイトルプレフィックスを作成
func (fs *FileSorter) createTitlePrefix(result *model.AnalysisResult) string {
	// 大人の場合
	if result.TargetAdult != "" {
		return fmt.Sprintf("[%s]", result.TargetAdult)
	}

	// 子供の場合
	if len(result.TargetChildren) > 0 && result.FiscalYear > 0 {
		if result.ResolvedEmoji != "" {
			return fmt.Sprintf("[%s]", result.ResolvedEmoji)
		}

		childName := result.TargetChildren[0]
		grade := fs.gradeManager.GetChildGrade(childName, result.FiscalYear)
		label, emoji := fs.gradeManager.GetGradeInfo(grade)

		if label != "" {
			return fmt.Sprintf("[%s]", label)
		}
		if emoji != "" {
			return fmt.Sprintf("[%s]", emoji)
		}
		return fmt.Sprintf("[%s]", childName)
	}

	if result.ChildName != "" {
		return fmt.Sprintf("[%s]", result.ChildName)
	}

	return ""
}

// analyzeDocument はドキュメントを解析
func (fs *FileSorter) analyzeDocument(ctx context.Context, data []byte, mimeType string, fileName string) (*model.AnalysisResult, error) {
	// プロンプトを作成
	prompt := fs.createAnalysisPrompt(fileName)

	return fs.aiRouter.AnalyzeDocument(ctx, data, mimeType, prompt, true)
}

// createAnalysisPrompt は解析プロンプトを作成
func (fs *FileSorter) createAnalysisPrompt(fileName string) string {
	// 子供の名寄せルール
	childAliasesStr := ""
	for name, aliases := range config.ChildAliases {
		childAliasesStr += fmt.Sprintf("%s: %s\n", name, strings.Join(aliases, ", "))
	}

	// 大人の名寄せルール
	adultAliasesStr := ""
	for name, aliases := range config.AdultAliases {
		adultAliasesStr += fmt.Sprintf("%s: %s\n", name, strings.Join(aliases, ", "))
	}

	return fmt.Sprintf(`
あなたは家庭内書類の整理アシスタントです。以下の画像を解析し、JSON形式で回答してください。

## お子様の名寄せルール
%s

## 大人の名寄せルール
%s

## 出力形式（必ずこのJSON形式で回答）
{
  "category": "カテゴリ名",
  "child_name": "お子様の名前（名寄せ後の正規名。複数または不明時は空文字）",
  "target_adult": "大人の名前（名寄せ後の正規名。書類の宛先・対象者が大人の場合。不明時は空文字）",
  "target_grade_class": "対象となる学年やクラス名（例：小2、くるみ組、1年生）。固有名詞がない場合に抽出",
  "sub_category": "サブカテゴリ（categoryが40_子供・教育の場合のみ）",
  "is_photo": false,
  "date": "YYYYMMDD形式の日付",
  "summary": "要約（15文字以内、ファイル名に使用）",
  "confidence_score": 0.0
}

## カテゴリ一覧
- 10_マネー・税務
- 20_プロジェクト・資産
- 30_ライフ・行政
- 40_子供・教育
- 50_写真・その他
- 90_ライブラリ

## サブカテゴリ（40_子供・教育の場合のみ使用）
- 01_お便り・スケジュール
- 02_提出・手続き・重要
- 03_記録・作品・成績

## 判断基準
- 書類の宛先や対象者が大人（祖父母、父、母など）の場合は target_adult に正規名を設定
- 子供関連の書類は child_name に設定し、categoryを「40_子供・教育」に
- 医療・健康関連、役所・公共関連は「30_ライフ・行政」に分類
- 金銭・銀行・税務関連は「10_マネー・税務」に分類
- is_photoがtrueの場合は、categoryを「50_写真・その他」にしてください
- 日付が不明な場合は本日の日付を使用してください
- confidence_scoreは0.0〜1.0の範囲で、解析結果の信頼度を示してください
- 学年やクラス名（「小2」「くるみ組」など）が記載されている場合は、target_grade_classに抽出してください

## ファイル名
%s
`, childAliasesStr, adultAliasesStr, fileName)
}

// isSupportedMimeType は対応しているMIMEタイプかチェック
func (fs *FileSorter) isSupportedMimeType(mimeType string) bool {
	for _, supported := range config.SupportedMimeTypes {
		if mimeType == supported {
			return true
		}
	}
	return false
}

// contains はスライスに要素が含まれているかチェック
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
