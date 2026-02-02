package service

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	"github.com/leo-sagawa/homedocmanager/internal/model"
	"golang.org/x/oauth2"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// DriveClient はGoogle Drive APIクライアント
type DriveClient struct {
	service     *drive.Service
	docsService *docs.Service
	oauthCreds  *OAuthCredentials
	folderCache map[string]string // key: "parentID:folderName", value: folderID
	folderMu    sync.Mutex
}

// NewDriveClient は新しいDriveClientを作成
func NewDriveClient(ctx context.Context) (*DriveClient, error) {
	// OAuth認証情報を取得
	creds, err := GetOAuthCredentials(ctx)
	if err != nil {
		log.Printf("Warning: Failed to get OAuth credentials for Drive, falling back to Service Account: %v", err)
		// サービスアカウント認証にフォールバック
		service, err := drive.NewService(ctx, option.WithScopes(drive.DriveScope))
		if err != nil {
			return nil, fmt.Errorf("failed to create drive service: %w", err)
		}
		docsService, err := docs.NewService(ctx, option.WithScopes(docs.DocumentsScope))
		if err != nil {
			return nil, fmt.Errorf("failed to create docs service: %w", err)
		}
		return &DriveClient{
			service:     service,
			docsService: docsService,
			folderCache: make(map[string]string),
		}, nil
	}

	// OAuth authenticated client
	if _, err := creds.GetAccessToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get access token for Drive: %w", err)
	}

	service, err := drive.NewService(ctx, option.WithTokenSource(&tokenSource{
		creds: creds,
		ctx:   ctx,
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to create drive service with OAuth: %w", err)
	}

	docsService, err := docs.NewService(ctx, option.WithTokenSource(&tokenSource{
		creds: creds,
		ctx:   ctx,
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to create docs service with OAuth: %w", err)
	}

	return &DriveClient{
		service:     service,
		docsService: docsService,
		oauthCreds:  creds,
		folderCache: make(map[string]string),
	}, nil
}

// tokenSource は oauth2.TokenSource インターフェースを実装
type tokenSource struct {
	creds *OAuthCredentials
	ctx   context.Context
}

func (ts *tokenSource) Token() (*oauth2.Token, error) {
	accessToken, err := ts.creds.GetAccessToken(ts.ctx)
	if err != nil {
		return nil, err
	}
	return &oauth2.Token{
		AccessToken: accessToken,
	}, nil
}

// GetFile はファイル情報を取得
func (c *DriveClient) GetFile(ctx context.Context, fileID string) (*model.FileInfo, error) {
	file, err := c.service.Files.Get(fileID).
		Fields("id, name, mimeType, parents").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get file: %w", err)
	}

	return &model.FileInfo{
		ID:       file.Id,
		Name:     file.Name,
		MimeType: file.MimeType,
		Parents:  file.Parents,
	}, nil
}

// DownloadFile はファイルをダウンロード（堅牢化版）
func (c *DriveClient) DownloadFile(ctx context.Context, fileID string) ([]byte, error) {
	maxRetries := 5
	for attempt := 0; attempt < maxRetries; attempt++ {
		log.Printf("ダウンロード開始（試行 %d/%d）: %s", attempt+1, maxRetries, fileID)

		// リトライ時はサービスを再構築（トークン切れなどの対策）
		if attempt > 0 {
			log.Println("サービスオブジェクトを再構築してリトライします")
			var service *drive.Service
			var err error
			if c.oauthCreds != nil {
				service, err = drive.NewService(ctx, option.WithTokenSource(&tokenSource{
					creds: c.oauthCreds,
					ctx:   ctx,
				}))
			} else {
				service, err = drive.NewService(ctx, option.WithScopes(drive.DriveScope))
			}
			if err != nil {
				log.Printf("サービス再構築エラー: %v", err)
				time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
				continue
			}
			c.service = service
		}

		resp, err := c.service.Files.Get(fileID).Download()
		if err != nil {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("download failed after %d attempts: %w", maxRetries, err)
			}
			log.Printf("ダウンロード失敗（リトライ待ち）: %v", err)
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
			continue
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			if attempt == maxRetries-1 {
				return nil, fmt.Errorf("failed to read response body: %w", err)
			}
			log.Printf("レスポンス読み取り失敗（リトライ待ち）: %v", err)
			time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
			continue
		}

		log.Printf("ダウンロード完了: %d bytes", len(data))
		return data, nil
	}

	return nil, fmt.Errorf("download failed after %d attempts", maxRetries)
}

// MoveFile はファイルを移動
func (c *DriveClient) MoveFile(ctx context.Context, fileID string, newParentID string) error {
	// 現在の親フォルダIDを取得
	file, err := c.GetFile(ctx, fileID)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	if len(file.Parents) == 0 {
		return fmt.Errorf("file has no parents")
	}

	currentParentID := file.Parents[0]

	// ファイルを移動
	_, err = c.service.Files.Update(fileID, nil).
		AddParents(newParentID).
		RemoveParents(currentParentID).
		Fields("id, parents").
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	log.Printf("ファイル移動成功: %s", fileID)
	return nil
}

// RenameFile はファイル名を変更
func (c *DriveClient) RenameFile(ctx context.Context, fileID string, newName string) error {
	_, err := c.service.Files.Update(fileID, &drive.File{
		Name: newName,
	}).SupportsAllDrives(true).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to rename file: %w", err)
	}

	log.Printf("ファイル名変更成功: %s", newName)
	return nil
}

// GetOrCreateFolder はフォルダを取得または作成（排他制御付き）
func (c *DriveClient) GetOrCreateFolder(ctx context.Context, folderName string, parentID string) (string, error) {
	cacheKey := fmt.Sprintf("%s:%s", parentID, folderName)

	// 排他制御で競合を防止
	c.folderMu.Lock()
	defer c.folderMu.Unlock()

	// キャッシュを確認
	if cachedID, exists := c.folderCache[cacheKey]; exists {
		return cachedID, nil
	}

	// フォルダが存在するか検索
	query := fmt.Sprintf("name='%s' and '%s' in parents and mimeType='application/vnd.google-apps.folder' and trashed=false", folderName, parentID)
	fileList, err := c.service.Files.List().
		Q(query).
		Fields("files(id, name)").
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("failed to search folder: %w", err)
	}

	// フォルダが存在する場合はキャッシュに追加して返す
	if len(fileList.Files) > 0 {
		folderID := fileList.Files[0].Id
		c.folderCache[cacheKey] = folderID
		return folderID, nil
	}

	// フォルダを新規作成
	folder := &drive.File{
		Name:     folderName,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}

	createdFolder, err := c.service.Files.Create(folder).
		Fields("id").
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("failed to create folder: %w", err)
	}

	// キャッシュに追加
	c.folderCache[cacheKey] = createdFolder.Id

	log.Printf("フォルダ作成成功: %s (%s)", folderName, createdFolder.Id)
	return createdFolder.Id, nil
}

// ListFilesInFolder はフォルダ内のファイル一覧を取得
func (c *DriveClient) ListFilesInFolder(ctx context.Context, folderID string, limit int) ([]*model.FileInfo, error) {
	query := fmt.Sprintf("'%s' in parents and trashed=false", folderID)
	call := c.service.Files.List().
		Q(query).
		PageSize(int64(limit)).
		Fields("files(id, name, mimeType, parents)").
		SupportsAllDrives(true).
		IncludeItemsFromAllDrives(true).
		Context(ctx)

	fileList, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var files []*model.FileInfo
	for _, f := range fileList.Files {
		files = append(files, &model.FileInfo{
			ID:       f.Id,
			Name:     f.Name,
			MimeType: f.MimeType,
			Parents:  f.Parents,
		})
	}

	return files, nil
}

// GetAbout はストレージ情報を取得
func (c *DriveClient) GetAbout(ctx context.Context) (map[string]interface{}, error) {
	about, err := c.service.About.Get().
		Fields("storageQuota, user").
		Context(ctx).
		Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get about info: %w", err)
	}

	result := map[string]interface{}{
		"storageQuota": about.StorageQuota,
		"user":         about.User,
	}

	return result, nil
}

// CleanupServiceAccountStorage はSAのストレージをクリーンアップ
func (c *DriveClient) CleanupServiceAccountStorage(ctx context.Context) (map[string]interface{}, error) {
	// クリーンアップロジックを実装（必要に応じて）
	// 現時点では簡易的な実装
	stats := map[string]interface{}{
		"cleaned": 0,
		"message": "Cleanup completed",
	}

	return stats, nil
}

// WatchInfo はWatch登録情報を保持
type WatchInfo struct {
	ChannelID      string
	ResourceID     string
	Expiration     int64
	StartPageToken string
}

// StartWatch はフォルダの変更監視を開始
func (c *DriveClient) StartWatch(ctx context.Context, webhookURL string) (*WatchInfo, error) {
	// 変更開始トークンを取得
	startPageToken, err := c.service.Changes.GetStartPageToken().Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get start page token: %w", err)
	}

	// チャンネルIDを生成（ユニークな識別子）
	channelID := fmt.Sprintf("homedocmanager-%d", time.Now().UnixNano())

	// 有効期限を7日後に設定（Drive APIの最大値）
	expiration := time.Now().Add(7 * 24 * time.Hour).UnixMilli()

	// Watch登録
	channel := &drive.Channel{
		Id:         channelID,
		Type:       "web_hook",
		Address:    webhookURL,
		Expiration: expiration,
	}

	watchedChannel, err := c.service.Changes.Watch(startPageToken.StartPageToken, channel).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to start watch: %w", err)
	}

	log.Printf("Watch started: channelID=%s, resourceID=%s, expiration=%d",
		watchedChannel.Id, watchedChannel.ResourceId, watchedChannel.Expiration)

	return &WatchInfo{
		ChannelID:      watchedChannel.Id,
		ResourceID:     watchedChannel.ResourceId,
		Expiration:     watchedChannel.Expiration,
		StartPageToken: startPageToken.StartPageToken,
	}, nil
}

// StopWatch は変更監視を停止
func (c *DriveClient) StopWatch(ctx context.Context, channelID, resourceID string) error {
	channel := &drive.Channel{
		Id:         channelID,
		ResourceId: resourceID,
	}

	err := c.service.Channels.Stop(channel).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to stop watch: %w", err)
	}

	log.Printf("Watch stopped: channelID=%s", channelID)
	return nil
}

// GetChanges は変更されたファイルIDを取得
func (c *DriveClient) GetChanges(ctx context.Context, pageToken string) ([]string, string, error) {
	var fileIDs []string
	nextPageToken := pageToken

	for {
		changes, err := c.service.Changes.List(nextPageToken).
			Fields("nextPageToken, newStartPageToken, changes(fileId, file(id, name, mimeType, parents, trashed))").
			Context(ctx).
			Do()
		if err != nil {
			return nil, pageToken, fmt.Errorf("failed to get changes: %w", err)
		}

		for _, change := range changes.Changes {
			// 削除されたファイルや、ファイル情報がないものはスキップ
			if change.File == nil || change.File.Trashed {
				continue
			}

			// PDFまたは画像のみ対象
			mimeType := change.File.MimeType
			if mimeType == "application/pdf" ||
				mimeType == "image/jpeg" ||
				mimeType == "image/png" ||
				mimeType == "image/gif" {
				fileIDs = append(fileIDs, change.FileId)
				log.Printf("Change detected: %s (%s)", change.File.Name, change.FileId)
			}
		}

		if changes.NewStartPageToken != "" {
			nextPageToken = changes.NewStartPageToken
			break
		}
		if changes.NextPageToken != "" {
			nextPageToken = changes.NextPageToken
		} else {
			break
		}
	}

	return fileIDs, nextPageToken, nil
}
