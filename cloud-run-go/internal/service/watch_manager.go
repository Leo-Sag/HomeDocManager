package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/leo-sagawa/homedocmanager/internal/model"
)

// WatchManager はDrive Watchの状態を管理
type WatchManager struct {
	driveClient *DriveClient
	fileSorter  *FileSorter
	webhookURL  string

	mu           sync.RWMutex
	currentWatch *WatchInfo
	pageToken    string
}

// NewWatchManager は新しいWatchManagerを作成
func NewWatchManager(driveClient *DriveClient, fileSorter *FileSorter, webhookURL string) *WatchManager {
	return &WatchManager{
		driveClient: driveClient,
		fileSorter:  fileSorter,
		webhookURL:  webhookURL,
	}
}

// StartWatch は監視を開始
func (wm *WatchManager) StartWatch(ctx context.Context) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	// 既存のWatchがあれば停止
	if wm.currentWatch != nil {
		if err := wm.driveClient.StopWatch(ctx, wm.currentWatch.ChannelID, wm.currentWatch.ResourceID); err != nil {
			log.Printf("Warning: failed to stop existing watch: %v", err)
		}
	}

	// 新しいWatchを開始
	watchInfo, err := wm.driveClient.StartWatch(ctx, wm.webhookURL)
	if err != nil {
		return err
	}

	wm.currentWatch = watchInfo
	wm.pageToken = watchInfo.StartPageToken

	return nil
}

// RenewWatch はWatchを更新
func (wm *WatchManager) RenewWatch(ctx context.Context) error {
	return wm.StartWatch(ctx)
}

// StopWatch は監視を停止
func (wm *WatchManager) StopWatch(ctx context.Context) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.currentWatch == nil {
		return nil
	}

	err := wm.driveClient.StopWatch(ctx, wm.currentWatch.ChannelID, wm.currentWatch.ResourceID)
	if err != nil {
		return err
	}

	wm.currentWatch = nil
	return nil
}

// HandleNotification は通知を受け取り変更を処理
func (wm *WatchManager) HandleNotification(ctx context.Context) (int, error) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	pageToken := wm.pageToken
	if pageToken == "" {
		log.Printf("Warning: pageToken is empty, attempting to fetch current start page token")
		startToken, err := wm.driveClient.service.Changes.GetStartPageToken().Context(ctx).Do()
		if err != nil {
			return 0, fmt.Errorf("failed to fetch start page token: %w", err)
		}
		pageToken = startToken.StartPageToken
		wm.pageToken = pageToken
		log.Printf("Successfully fetched new pageToken: %s", pageToken)
	}

	// 変更を取得
	fileIDs, nextPageToken, err := wm.driveClient.GetChanges(ctx, pageToken)
	if err != nil {
		return 0, err
	}

	// ページトークンを更新
	wm.pageToken = nextPageToken

	// ロックを解除して処理（時間のかかる処理のため）
	// ただし、同じファイルIDが重複して処理されるのを防ぐため、処理中リストなどが必要かもしれない
	// 今回は簡易的に一度ロックを解いて処理する
	wm.mu.Unlock()

	// 各ファイルを処理
	processed := 0
	for _, fileID := range fileIDs {
		log.Printf("Processing file from notification: %s", fileID)
		result := wm.fileSorter.ProcessFile(ctx, fileID)
		if result == model.ProcessResultProcessed {
			processed++
		}
	}

	// 終了時に再度ロックを取得（deferでのUnlockに対応するため）
	wm.mu.Lock()
	return processed, nil
}

// GetStatus は現在のWatch状態を返す
func (wm *WatchManager) GetStatus() map[string]interface{} {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	status := map[string]interface{}{
		"active": wm.currentWatch != nil,
	}

	if wm.currentWatch != nil {
		expirationTime := time.UnixMilli(wm.currentWatch.Expiration)
		status["channelId"] = wm.currentWatch.ChannelID
		status["resourceId"] = wm.currentWatch.ResourceID
		status["expiration"] = expirationTime.Format(time.RFC3339)
		status["expiresIn"] = time.Until(expirationTime).String()
	}

	return status
}
