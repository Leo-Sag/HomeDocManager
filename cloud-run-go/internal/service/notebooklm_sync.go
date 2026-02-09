package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/leo-sagawa/homedocmanager/internal/config"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
)

// NotebookLMSync はNotebookLM同期サービス
type NotebookLMSync struct {
	driveClient *DriveClient
	mu          sync.Mutex
}

const processedMarker = "notebooklm_synced"

// NewNotebookLMSync は新しいNotebookLMSyncを作成
func NewNotebookLMSync(ctx context.Context, driveClient *DriveClient) (*NotebookLMSync, error) {
	return &NotebookLMSync{
		driveClient: driveClient,
	}, nil
}

// ShouldSync は同期対象のカテゴリ・サブカテゴリかどうかを判定（除外ベース）
func (ns *NotebookLMSync) ShouldSync(category, subCategory string) bool {
	// カテゴリ全体が除外されているか
	if config.NotebookLMSyncExcludeCategories[category] {
		return false
	}
	// サブカテゴリが除外されているか
	if excludedSubs, ok := config.NotebookLMSyncExcludeSubCategories[category]; ok {
		for _, sub := range excludedSubs {
			if sub == subCategory {
				return false
			}
		}
	}
	// NotebookLMカテゴリマッピングが存在するか
	if _, exists := config.NotebookLMCategoryMap[category]; !exists {
		return false
	}
	return true
}

// SyncFile はファイルをNotebookLMに同期
func (ns *NotebookLMSync) SyncFile(ctx context.Context, fileID, fileName, notebookCategory, ocrText string, facts []string, summary, dateStr string, fiscalYear int) error {
	// 日付をフォーマット
	formattedDate := formatDateForNotebook(dateStr)

	// 順次処理を保証するためロックを取得
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// 累積ドキュメントを取得または作成
	docID, mimeType, err := ns.getOrCreateAccumulatedDoc(ctx, fiscalYear, notebookCategory)
	if err != nil {
		return fmt.Errorf("累積ドキュメント取得/作成失敗: %w", err)
	}

	// ドキュメントに追記
	entryText := ns.formatEntry(formattedDate, fileName, fileID, ocrText, facts, summary, notebookCategory)
	if err := ns.appendToDoc(ctx, docID, mimeType, entryText); err != nil {
		return fmt.Errorf("ドキュメント追記失敗: %w", err)
	}

	// 元ファイルに同期済みマーカーを設定
	ns.markAsSynced(ctx, fileID)

	log.Printf("NotebookLM同期完了: %s → %d年度_%s", fileName, fiscalYear, notebookCategory)
	return nil
}

// formatDateForNotebook はYYYYMMDD形式をYYYY/MM/DD形式に変換
func formatDateForNotebook(dateStr string) string {
	if len(dateStr) != 8 {
		return time.Now().Format("2006-01-02")
	}
	// 要件 YYYY-MM-DD
	return fmt.Sprintf("%s-%s-%s", dateStr[:4], dateStr[4:6], dateStr[6:8])
}

// formatEntry はエントリテキストを要件に基づきフォーマット
func (ns *NotebookLMSync) formatEntry(formattedDate, fileName, fileID, ocrText string, facts []string, summary, notebookCategory string) string {
	fileURL := fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID)

	// facts を文字列に変換
	factsStr := ""
	if len(facts) > 0 {
		for _, fact := range facts {
			factsStr += fmt.Sprintf("- %s\n", fact)
		}
	} else {
		factsStr = "- （抽出なし）\n"
	}

	// summary部分（空なら省略）
	summarySection := ""
	if summary != "" {
		summarySection = fmt.Sprintf("\n要約（自動・要確認）:\n%s\n", summary)
	}

	return fmt.Sprintf(`---
## %s

カテゴリ: %s
最終更新: %s
元ファイル: %s

重要情報（抽出・推測なし）:
%s%s
本文（OCR原文）:
%s

`, fileName, notebookCategory, formattedDate, fileURL, factsStr, summarySection, ocrText)
}

// getOrCreateAccumulatedDoc は年度別・カテゴリ別統合ドキュメントを取得または作成
func (ns *NotebookLMSync) getOrCreateAccumulatedDoc(ctx context.Context, fiscalYear int, notebookCategory string) (string, string, error) {
	syncFolderID := config.FolderIDs["NOTEBOOKLM_SYNC"]
	if syncFolderID == "" {
		return "", "", fmt.Errorf("NOTEBOOKLM_SYNCフォルダIDが設定されていません")
	}

	docName := fmt.Sprintf("%d年度_%s", fiscalYear, notebookCategory)

	// 既存のドキュメントを検索
	docID, mimeType, err := ns.findDocByName(ctx, docName, syncFolderID)
	if err != nil {
		return "", "", err
	}
	if docID != "" {
		return docID, mimeType, nil
	}

	// 新規作成
	docID, err = ns.createUnifiedDoc(ctx, docName, syncFolderID, fiscalYear, notebookCategory)
	if err != nil {
		return "", "", err
	}

	return docID, "application/vnd.google-apps.document", nil
}

// findDocByName はフォルダ内でファイルを名前で検索し、IDとMimeTypeを返す
func (ns *NotebookLMSync) findDocByName(ctx context.Context, docName, parentID string) (string, string, error) {
	query := fmt.Sprintf("name='%s' and '%s' in parents and trashed=false", docName, parentID)

	fileList, err := ns.driveClient.service.Files.List().
		Q(query).
		Fields("files(id, name, mimeType)").
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return "", "", fmt.Errorf("ドキュメント検索エラー: %w", err)
	}

	if len(fileList.Files) > 0 {
		return fileList.Files[0].Id, fileList.Files[0].MimeType, nil
	}
	return "", "", nil
}

// createUnifiedDoc は新しい統合ドキュメント（Google Doc）を作成
func (ns *NotebookLMSync) createUnifiedDoc(ctx context.Context, docName, parentID string, fiscalYear int, notebookCategory string) (string, error) {
	// Google Doc として作成
	file := &drive.File{
		Name:     docName,
		MimeType: "application/vnd.google-apps.document",
		Parents:  []string{parentID},
	}

	createdDoc, err := ns.driveClient.oauthDriveService.Files.Create(file).
		Fields("id").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("ドキュメント作成失敗: %w", err)
	}

	docID := createdDoc.Id

	// 初期ヘッダーを挿入
	headerText := fmt.Sprintf("# %d年度 %s\n\nこのファイルは NotebookLM 用に自動生成された書類OCRテキストの統合ファイルです。\n\n", fiscalYear, notebookCategory)

	requests := []*docs.Request{
		{
			InsertText: &docs.InsertTextRequest{
				Location: &docs.Location{Index: 1},
				Text:     headerText,
			},
		},
	}
	_, err = ns.driveClient.docsService.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
		Requests: requests,
	}).Context(ctx).Do()
	if err != nil {
		log.Printf("初期ヘッダー挿入失敗 (ID: %s): %v", docID, err)
		// 致命的ではないとして続行
	}

	// オーナー権限を転送（設定されている場合）
	if config.NotebookLMOwnerEmail != "" {
		permission := &drive.Permission{
			Role:         "owner",
			Type:         "user",
			EmailAddress: config.NotebookLMOwnerEmail,
		}
		_, err := ns.driveClient.oauthDriveService.Permissions.Create(docID, permission).
			TransferOwnership(true).
			Context(ctx).
			Do()
		if err != nil {
			log.Printf("オーナー権限の転送に失敗しました（容量制限に注意）: %v", err)
		} else {
			log.Printf("ファイルのオーナー権限を転送しました: %s", config.NotebookLMOwnerEmail)
		}
	}

	log.Printf("統合ドキュメント作成: %s (ID: %s)", docName, docID)
	return docID, nil
}

// appendToDoc はファイルの末尾にテキストを追記 (Docs APIを使用)
func (ns *NotebookLMSync) appendToDoc(ctx context.Context, docID, mimeType, text string) error {
	if mimeType != "application/vnd.google-apps.document" {
		return fmt.Errorf("非対応のMIMEタイプです: %s", mimeType)
	}

	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		// EndOfSegmentLocation を使用して末尾に追記（より安定）
		requests := []*docs.Request{
			{
				InsertText: &docs.InsertTextRequest{
					EndOfSegmentLocation: &docs.EndOfSegmentLocation{},
					Text:                 text,
				},
			},
		}

		// 実行
		_, err := ns.driveClient.docsService.Documents.BatchUpdate(docID, &docs.BatchUpdateDocumentRequest{
			Requests: requests,
		}).Context(ctx).Do()

		if err == nil {
			return nil
		}

		if ns.isRetryable(err) && attempt < maxRetries-1 {
			log.Printf("追記リトライ中 (%d/%d): %v", attempt+1, maxRetries, err)
			ns.backoff(attempt)
			continue
		}

		return fmt.Errorf("ドキュメント追記失敗: %w", err)
	}

	return fmt.Errorf("最大リトライ回数を超えました")
}

// isRetryable はリトライすべきエラーかどうかを判定
func (ns *NotebookLMSync) isRetryable(err error) bool {
	if gerr, ok := err.(*googleapi.Error); ok {
		return gerr.Code == 409 || gerr.Code == 429 || gerr.Code >= 500
	}
	return false
}

// backoff は指数バックオフ
func (ns *NotebookLMSync) backoff(attempt int) {
	duration := time.Duration(1<<uint(attempt)) * time.Second
	duration += time.Duration(rand.Intn(1000)) * time.Millisecond
	time.Sleep(duration)
}

// markAsSynced はファイルを同期済みとしてマーク
func (ns *NotebookLMSync) markAsSynced(ctx context.Context, fileID string) {
	file := &drive.File{
		Properties: map[string]string{
			processedMarker: "true",
		},
	}

	_, err := ns.driveClient.service.Files.Update(fileID, file).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		log.Printf("同期マーキングエラー: %v", err)
	}
}

// IsAlreadySynced はファイルが既に同期済みかチェック
func (ns *NotebookLMSync) IsAlreadySynced(ctx context.Context, fileID string) bool {
	file, err := ns.driveClient.service.Files.Get(fileID).
		Fields("properties").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		log.Printf("同期状態チェックエラー: %v", err)
		return false
	}

	return file.Properties[processedMarker] == "true"
}
