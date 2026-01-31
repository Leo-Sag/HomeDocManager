package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/leo-sagawa/homedocmanager/internal/config"
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

// ShouldSync は同期対象のカテゴリかどうかを判定
func (ns *NotebookLMSync) ShouldSync(category string) bool {
	for _, c := range config.NotebookLMSyncCategories {
		if c == category {
			return true
		}
	}
	return false
}

// SyncFile はファイルをNotebookLMに同期
func (ns *NotebookLMSync) SyncFile(ctx context.Context, fileID, fileName, category, ocrText, dateStr string, fiscalYear int) error {
	if !ns.ShouldSync(category) {
		log.Printf("カテゴリ %s は同期対象外です", category)
		return nil
	}

	// 日付をフォーマット
	formattedDate := formatDateForNotebook(dateStr)

	// 順次処理を保証するためロックを取得
	ns.mu.Lock()
	defer ns.mu.Unlock()

	// 累積ドキュメントを取得または作成
	docID, mimeType, err := ns.getOrCreateAccumulatedDoc(ctx, fiscalYear)
	if err != nil {
		return fmt.Errorf("累積ドキュメント取得/作成失敗: %w", err)
	}

	// ドキュメントに追記
	entryText := ns.formatEntry(formattedDate, fileName, fileID, ocrText, category)
	if err := ns.appendToDoc(ctx, docID, mimeType, entryText); err != nil {
		return fmt.Errorf("ドキュメント追記失敗: %w", err)
	}

	// 元ファイルに同期済みマーカーを設定
	ns.markAsSynced(ctx, fileID)

	log.Printf("NotebookLM同期完了: %s → %d年度_全記録", fileName, fiscalYear)
	return nil
}

// formatDateForNotebook はYYYYMMDD形式をYYYY/MM/DD形式に変換
func formatDateForNotebook(dateStr string) string {
	if len(dateStr) != 8 {
		return time.Now().Format("2006/01/02")
	}
	return fmt.Sprintf("%s/%s/%s", dateStr[:4], dateStr[4:6], dateStr[6:8])
}

// formatEntry はエントリテキストをMarkdown形式でフォーマット
func (ns *NotebookLMSync) formatEntry(formattedDate, fileName, fileID, ocrText, category string) string {
	fileURL := fmt.Sprintf("https://drive.google.com/file/d/%s/view", fileID)

	return fmt.Sprintf(`
---

## 📄 %s - [%s] %s

🔗 [元ファイルを開く](%s)

%s

`, formattedDate, category, fileName, fileURL, ocrText)
}

// getOrCreateAccumulatedDoc は年度別統合ドキュメントを取得または作成
func (ns *NotebookLMSync) getOrCreateAccumulatedDoc(ctx context.Context, fiscalYear int) (string, string, error) {
	syncFolderID := config.FolderIDs["NOTEBOOKLM_SYNC"]
	if syncFolderID == "" {
		return "", "", fmt.Errorf("NOTEBOOKLM_SYNCフォルダIDが設定されていません")
	}

	docName := fmt.Sprintf("%d年度_全記録", fiscalYear)

	// 既存のドキュメントを検索
	docID, mimeType, err := ns.findDocByName(ctx, docName, syncFolderID)
	if err != nil {
		return "", "", err
	}
	if docID != "" {
		return docID, mimeType, nil
	}

	// 新規作成
	docID, err = ns.createUnifiedDoc(ctx, docName, syncFolderID, fiscalYear)
	if err != nil {
		return "", "", err
	}

	return docID, "text/markdown", nil
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

// createUnifiedDoc は新しい統合ドキュメントを作成
func (ns *NotebookLMSync) createUnifiedDoc(ctx context.Context, docName, parentID string, fiscalYear int) (string, error) {
	// Markdownファイルとして作成
	file := &drive.File{
		Name:     docName,
		MimeType: "text/markdown",
		Parents:  []string{parentID},
	}

	// ヘッダーテキストを初期内容として設定
	headerText := fmt.Sprintf("# %d年度 全記録\n\n> このファイルは NotebookLM 用に自動生成された書類OCRテキストの統合ファイルです。\n> 各エントリには [カテゴリ名] が付与されています。\n\n", fiscalYear)

	createdDoc, err := ns.driveClient.service.Files.Create(file).
		Media(bytes.NewReader([]byte(headerText)), googleapi.ContentType("text/markdown")).
		Fields("id").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("ドキュメント作成失敗: %w", err)
	}

	docID := createdDoc.Id

	// オーナー権限を転送（設定されている場合）
	if config.NotebookLMOwnerEmail != "" {
		permission := &drive.Permission{
			Role:         "owner",
			Type:         "user",
			EmailAddress: config.NotebookLMOwnerEmail,
		}
		_, err := ns.driveClient.service.Permissions.Create(docID, permission).
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

// appendToDoc はファイルの末尾にテキストを追記
func (ns *NotebookLMSync) appendToDoc(ctx context.Context, docID, mimeType, text string) error {
	var currentContent []byte
	var err error

	if mimeType == "application/vnd.google-apps.document" {
		// Googleドキュメントの場合は Export
		resp, err := ns.driveClient.service.Files.Export(docID, "text/plain").Context(ctx).Download()
		if err != nil {
			return fmt.Errorf("ドキュメントエクスポート失敗: %w", err)
		}
		defer resp.Body.Close()
		currentContent, err = io.ReadAll(resp.Body)
	} else {
		// それ以外（Markdown等）は通常ダウンロード
		resp, err := ns.driveClient.service.Files.Get(docID).
			SupportsAllDrives(true).
			Download()
		if err != nil {
			return fmt.Errorf("ファイルダウンロード失敗: %w", err)
		}
		defer resp.Body.Close()
		currentContent, err = io.ReadAll(resp.Body)
	}

	if err != nil && err != io.EOF {
		return fmt.Errorf("内容読み込み失敗: %w", err)
	}

	// 新しい内容を追加
	newContent := string(currentContent) + text

	if mimeType == "application/vnd.google-apps.document" {
		// Googleドキュメントの更新は今のところ text/plain での更新が難しいため、
		// 追記ではなく、Google Docs API を使うか、一旦現状のまま（Markdown優先）とする。
		// ここでは、ユーザーの希望通り Markdown 優先なので、Docの場合はログを出して何もしないか、
		// あるいは上書きしてしまう検討が必要。
		// ロバスト性のため、Docの場合はスキップして新規作成に誘導するのが安全だが、
		// ここでは一旦更新を試みる。
		log.Printf("Warning: 既存のファイルがGoogleドキュメントです。上書きまたはエラーの可能性があります。")
	}

	// ファイルを更新（MimeTypeを維持しつつメディアを更新）
	_, err = ns.driveClient.service.Files.Update(docID, nil).
		Media(bytes.NewReader([]byte(newContent)), googleapi.ContentType(mimeType)).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("ファイル更新失敗: %w", err)
	}

	return nil
}

// markAsSynced はファイルを同期済みとしてマーク
func (ns *NotebookLMSync) markAsSynced(ctx context.Context, fileID string) {
	file := &drive.File{
		Properties: map[string]string{
			processedMarker: "true",
		},
	}

	_, err := ns.driveClient.service.Files.Update(fileID, file).
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
		Context(ctx).
		Do()
	if err != nil {
		log.Printf("同期状態チェックエラー: %v", err)
		return false
	}

	return file.Properties[processedMarker] == "true"
}
