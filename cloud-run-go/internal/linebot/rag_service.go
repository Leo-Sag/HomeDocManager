package linebot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/leo-sagawa/homedocmanager/internal/config"
	"github.com/leo-sagawa/homedocmanager/internal/model"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// DriveClientInterface は循環参照を避けるための最小限のインターフェース
type DriveClientInterface interface {
	ListFilesInFolder(ctx context.Context, folderID string, limit int) ([]*model.FileInfo, error)
	GetDriveService() *drive.Service
	GetDocsService() *docs.Service
}

// RAGService はGoogle DocsからテキストをFetchし、Gemini APIで回答を生成するサービス
type RAGService struct {
	driveClient     DriveClientInterface
	geminiClient    *genai.Client
	userMap         map[string]string
	mu              sync.RWMutex // ユーザーマップおよびキャッシュ更新用
	documentIDs     []string     // 個別に指定されたドキュメントID
	sourceFolderIDs []string     // 自動走査対象のフォルダID
	modelName       string
	systemPrompt    string

	// キャッシュ
	docCache   string
	cacheValid bool
	lastSync   time.Time
	lastCheck  time.Time
}

// RAGUserSettings はJSONファイルから読み込むユーザー設定
type RAGUserSettings struct {
	UserMap            map[string]string `json:"user_map"`
	RAGDocumentIDs     []string          `json:"rag_document_ids"`
	RAGSourceFolderIDs []string          `json:"rag_source_folder_ids"`
	RAGSettings        struct {
		Model                string  `json:"model"`
		Temperature          float32 `json:"temperature"`
		SystemPromptTemplate string  `json:"system_prompt_template"`
	} `json:"rag_settings"`
}

// NewRAGService は新しいRAGサービスを作成
// geminiAPIKey が空の場合はnilを返す（RAG機能無効）
func NewRAGService(ctx context.Context, geminiAPIKey string, settingsPath string, driveClient DriveClientInterface) (*RAGService, error) {
	if geminiAPIKey == "" {
		return nil, nil // RAG機能無効
	}

	// 設定ファイルを読み込み
	settings, err := loadRAGUserSettings(settingsPath)
	if err != nil {
		// 設定ファイルがなくてもデフォルト値で動作
		settings = &RAGUserSettings{
			UserMap:            config.LineUserMap,
			RAGDocumentIDs:     config.RAGDocumentIDs,
			RAGSourceFolderIDs: config.LineRAGSourceFolderIDs,
		}
		settings.RAGSettings.Model = config.GeminiModelsConfig.LineRAG
		settings.RAGSettings.Temperature = 0.0
		settings.RAGSettings.SystemPromptTemplate = defaultSystemPrompt()
	}

	// ドキュメントIDとソースフォルダIDが両方空の場合はRAG機能無効
	if len(settings.RAGDocumentIDs) == 0 && len(settings.RAGSourceFolderIDs) == 0 {
		return nil, nil
	}

	// Google Docs APIクライアント（デフォルト認証を使用）
	// docsSvc, err := docs.NewService(ctx) // docsServiceはdriveClientから取得するため不要
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to create docs service: %w", err)
	// }

	// Gemini クライアント
	geminiClient, err := genai.NewClient(ctx, option.WithAPIKey(geminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini client: %w", err)
	}

	modelName := settings.RAGSettings.Model
	if modelName == "" {
		modelName = config.GeminiModelsConfig.LineRAG
	}

	systemPrompt := settings.RAGSettings.SystemPromptTemplate
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt()
	}

	return &RAGService{
		driveClient:     driveClient,
		geminiClient:    geminiClient,
		userMap:         mergeUserMaps(config.LineUserMap, settings.UserMap),
		documentIDs:     settings.RAGDocumentIDs,
		sourceFolderIDs: settings.RAGSourceFolderIDs,
		modelName:       modelName,
		systemPrompt:    systemPrompt,
		cacheValid:      false,
	}, nil
}

func loadRAGUserSettings(path string) (*RAGUserSettings, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var settings RAGUserSettings
	if err := json.Unmarshal(b, &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func mergeUserMaps(base, override map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}

func defaultSystemPrompt() string {
	return "あなたは家族のアシスタントです。提供されたコンテキストのみに基づいて、ユーザーの質問に日本語で回答してください。" +
		"現在のユーザーは{user_name}です。{user_name}に関連する情報を優先してください。" +
		"また、子供（明日香、遥香、文香、ビクトル、ミハイル、アンナ）に関する情報もすべて参照可能です。" +
		"回答がコンテキスト内にない場合は、「該当する情報がドキュメント内に見つかりませんでした。」と明示してください。"
}

// GenerateAnswer はユーザークエリに対する回答を生成
func (r *RAGService) GenerateAnswer(ctx context.Context, userID, query string) (string, error) {
	// キャッシュの準備
	docContext, err := r.getOrSyncCache(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to sync documents: %w", err)
	}

	r.mu.RLock()
	userName := r.userMap[userID]
	modelName := r.modelName
	systemPromptTemplate := r.systemPrompt
	r.mu.RUnlock()

	if userName == "" {
		userName = "家族メンバー"
	}

	// システムプロンプトにユーザー名を埋め込み
	systemPrompt := strings.ReplaceAll(systemPromptTemplate, "{user_name}", userName)

	// Gemini モデル設定
	model := r.geminiClient.GenerativeModel(modelName)
	model.SetTemperature(0.0)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(systemPrompt),
		},
	}

	// プロンプト構築
	prompt := fmt.Sprintf("Context:\n%s\n\nQuestion: %s", docContext, query)

	// 回答生成
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "回答を生成できませんでした。", nil
	}

	// 回答テキストを抽出
	return fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0]), nil
}

// getOrSyncCache はキャッシュが有効なら返し、無効なら同期して返す
func (r *RAGService) getOrSyncCache(ctx context.Context) (string, error) {
	now := time.Now()

	r.mu.RLock()
	cacheValid := r.cacheValid
	lastCheck := r.lastCheck
	r.mu.RUnlock()

	// 5分ごとにフォルダの変更をチェック
	if cacheValid && now.Sub(lastCheck) > 5*time.Minute {
		if changed, err := r.checkFoldersForChanges(ctx); err == nil && changed {
			log.Printf("[RAG] Remote folder change detected, invalidating cache")
			r.InvalidateCache()
			cacheValid = false
		} else {
			r.mu.Lock()
			r.lastCheck = now
			r.mu.Unlock()
		}
	}

	if cacheValid {
		r.mu.RLock()
		cache := r.docCache
		r.mu.RUnlock()
		return cache, nil
	}

	// キャッシュ無効な場合は同期
	return r.RefreshCache(ctx)
}

// checkFoldersForChanges は対象フォルダのいずれかに変更があったか確認
func (r *RAGService) checkFoldersForChanges(ctx context.Context) (bool, error) {
	driveSvc := r.driveClient.GetDriveService()
	r.mu.RLock()
	lastSync := r.lastSync
	folderIDs := r.sourceFolderIDs
	r.mu.RUnlock()

	for _, id := range folderIDs {
		f, err := driveSvc.Files.Get(id).Fields("modifiedTime").Context(ctx).Do()
		if err != nil {
			continue
		}
		modTime, err := time.Parse(time.RFC3339, f.ModifiedTime)
		if err != nil {
			continue
		}
		if modTime.After(lastSync) {
			return true, nil
		}
	}
	return false, nil
}

// InvalidateCache はキャッシュを無効化する（外部から呼ぶ）
func (r *RAGService) InvalidateCache() {
	r.mu.Lock()
	defer r.mu.Unlock() // NOTE: h -> r の間違いを修正
	r.cacheValid = false
	log.Printf("[RAG] Cache invalidated")
}

// RefreshCache は全ドキュメントを再スキャンしてキャッシュを更新
func (r *RAGService) RefreshCache(ctx context.Context) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	log.Printf("[RAG] Refreshing cache from documents and folders...")

	var allText strings.Builder
	docIDs := make(map[string]bool)

	// 1. 直接指定された個別のDoc IDを取得
	for _, id := range r.documentIDs {
		docIDs[id] = true
	}

	// 2. フォルダ内を再帰的に走査（簡易版：直下のみを想定）
	for _, folderID := range r.sourceFolderIDs {
		files, err := r.driveClient.ListFilesInFolder(ctx, folderID, 100)
		if err != nil {
			log.Printf("[RAG] Failed to list files in folder %s: %v", folderID, err)
			continue
		}

		for _, f := range files {
			// Googleドキュメントのみ対象
			if f.MimeType == "application/vnd.google-apps.document" {
				docIDs[f.ID] = true
			}
		}
	}

	// 3. 全てのドキュメントからテキストを抽出
	docsSvc := r.driveClient.GetDocsService()
	for id := range docIDs {
		text, err := r.fetchSingleDocumentText(ctx, docsSvc, id)
		if err != nil {
			log.Printf("[RAG] Failed to fetch text for doc %s: %v", id, err)
			continue
		}
		allText.WriteString(text)
		allText.WriteString("\n---\n")
	}

	r.docCache = allText.String()
	r.cacheValid = true
	r.lastSync = time.Now()

	log.Printf("[RAG] Cache refreshed: %d domains/docs processed, %d chars", len(docIDs), len(r.docCache))
	return r.docCache, nil
}

// fetchSingleDocumentText は単一のドキュメントからテキストを抽出
func (r *RAGService) fetchSingleDocumentText(ctx context.Context, docsSvc *docs.Service, docID string) (string, error) {
	doc, err := docsSvc.Documents.Get(docID).Context(ctx).Do()
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== %s ===\n", doc.Title))

	for _, element := range doc.Body.Content {
		if element.Paragraph != nil {
			for _, pe := range element.Paragraph.Elements {
				if pe.TextRun != nil && pe.TextRun.Content != "" {
					sb.WriteString(pe.TextRun.Content)
				}
			}
		}
	}
	return sb.String(), nil
}

// UpdateUser はUserIDと名前を動的に紐付ける
func (r *RAGService) UpdateUser(userID, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.userMap == nil {
		r.userMap = make(map[string]string)
	}
	r.userMap[userID] = name
}

// IsUserKnown はUserIDが既にマップにあるか確認
func (r *RAGService) IsUserKnown(userID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.userMap[userID]
	return ok
}

// IdentifyUserByDisplayName は表示名から大人メンバーの名前を特定
func (r *RAGService) IdentifyUserByDisplayName(displayName string) string {
	for name, aliases := range config.AdultAliases {
		if name == displayName {
			return name
		}
		for _, alias := range aliases {
			if strings.EqualFold(alias, displayName) {
				return name
			}
		}
	}
	return ""
}

// Close はリソースを解放
func (r *RAGService) Close() error {
	if r.geminiClient != nil {
		return r.geminiClient.Close()
	}
	return nil
}
