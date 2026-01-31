package handler

import (
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/leo-sagawa/homedocmanager/internal/config"
	"github.com/leo-sagawa/homedocmanager/internal/model"
	"github.com/leo-sagawa/homedocmanager/internal/service"
)

// PubSubHandler はPub/Subメッセージを処理するハンドラー
type PubSubHandler struct {
	services     *service.Services
	watchManager *service.WatchManager
}

// NewPubSubHandler は新しいPubSubHandlerを作成
func NewPubSubHandler(services *service.Services, watchManager *service.WatchManager) *PubSubHandler {
	return &PubSubHandler{
		services:     services,
		watchManager: watchManager,
	}
}

// HandlePubSub はPub/Subトリガーを処理
func (h *PubSubHandler) HandlePubSub(c *gin.Context) {
	var message model.PubSubMessage
	if err := c.ShouldBindJSON(&message); err != nil {
		log.Printf("Invalid Pub/Sub message format: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad Request"})
		return
	}

	// Pub/Subメッセージをデコード
	data, err := base64.StdEncoding.DecodeString(message.Message.Data)
	if err != nil {
		log.Printf("Failed to decode message data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad Request"})
		return
	}

	var fileData model.FileData
	if err := json.Unmarshal(data, &fileData); err != nil {
		log.Printf("Failed to unmarshal file data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad Request"})
		return
	}

	if fileData.FileID == "" {
		log.Printf("file_id not found in message")
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_id is required"})
		return
	}

	log.Printf("Processing file: %s", fileData.FileID)

	// ファイル処理を実行
	result := h.services.FileSorter.ProcessFile(c.Request.Context(), fileData.FileID)

	if result == model.ProcessResultProcessed || result == model.ProcessResultSkipped {
		log.Printf("File processed successfully (%s): %s", result, fileData.FileID)
		c.JSON(http.StatusOK, gin.H{"status": "OK"})
		return
	}

	log.Printf("File processing failed: %s", fileData.FileID)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
}

// HealthCheck はヘルスチェックエンドポイント
func (h *PubSubHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

// TestEndpoint はテスト用エンドポイント（手動トリガー）
func (h *PubSubHandler) TestEndpoint(c *gin.Context) {
	var req struct {
		FileID string `json:"file_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_id is required"})
		return
	}

	log.Printf("Test processing file: %s", req.FileID)
	result := h.services.FileSorter.ProcessFile(c.Request.Context(), req.FileID)

	if result == model.ProcessResultProcessed || result == model.ProcessResultSkipped {
		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"result":  string(result),
			"file_id": req.FileID,
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"status":  "failed",
		"file_id": req.FileID,
	})
}

// AdminInfo はストレージ情報を取得
func (h *PubSubHandler) AdminInfo(c *gin.Context) {
	about, err := h.services.DriveClient.GetAbout(c.Request.Context())
	if err != nil {
		log.Printf("Error getting info: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "OK",
		"about":  about,
	})
}

// AdminCleanup はSAのストレージクリーンアップを実行
func (h *PubSubHandler) AdminCleanup(c *gin.Context) {
	stats, err := h.services.DriveClient.CleanupServiceAccountStorage(c.Request.Context())
	if err != nil {
		log.Printf("Error executing cleanup: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "OK",
		"stats":  stats,
	})
}

// TriggerInbox はInboxフォルダ内の全ファイルを処理
func (h *PubSubHandler) TriggerInbox(c *gin.Context) {
	inboxID := config.FolderIDs["SOURCE"]
	files, err := h.services.DriveClient.ListFilesInFolder(c.Request.Context(), inboxID, 50)
	if err != nil {
		log.Printf("Error listing files: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	results := map[string]interface{}{
		"processed": 0,
		"errors":    0,
		"details":   []map[string]interface{}{},
	}

	for _, file := range files {
		fileID := file.ID
		fileName := file.Name
		log.Printf("Inbox scan processing: %s (%s)", fileName, fileID)

		result := h.services.FileSorter.ProcessFile(c.Request.Context(), fileID)
		detail := map[string]interface{}{
			"id":     fileID,
			"name":   fileName,
			"result": string(result),
		}

		if result == model.ProcessResultProcessed || result == model.ProcessResultSkipped {
			results["processed"] = results["processed"].(int) + 1
		} else {
			results["errors"] = results["errors"].(int) + 1
		}

		results["details"] = append(results["details"].([]map[string]interface{}), detail)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"results": results,
	})
}

// HandleDriveWebhook はGoogle Driveからの変更通知を処理
func (h *PubSubHandler) HandleDriveWebhook(c *gin.Context) {
	// Drive APIからの通知ヘッダーを確認
	channelID := c.GetHeader("X-Goog-Channel-ID")
	resourceState := c.GetHeader("X-Goog-Resource-State")

	log.Printf("Drive webhook received: channelID=%s, state=%s", channelID, resourceState)

	// syncは初期確認なのでスキップ
	if resourceState == "sync" {
		log.Printf("Sync notification received, acknowledging")
		c.JSON(http.StatusOK, gin.H{"status": "sync acknowledged"})
		return
	}

	// 変更があった場合は処理
	if resourceState == "change" {
		if h.watchManager == nil {
			log.Printf("WatchManager not initialized")
			c.JSON(http.StatusOK, gin.H{"status": "OK", "message": "watchManager not available"})
			return
		}

		processed, err := h.watchManager.HandleNotification(c.Request.Context())
		if err != nil {
			log.Printf("Error processing notification: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		log.Printf("Processed %d files from notification", processed)
		c.JSON(http.StatusOK, gin.H{
			"status":    "OK",
			"processed": processed,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

// WatchStart はWatch監視を開始
func (h *PubSubHandler) WatchStart(c *gin.Context) {
	if h.watchManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "watchManager not initialized"})
		return
	}

	if err := h.watchManager.StartWatch(c.Request.Context()); err != nil {
		log.Printf("Failed to start watch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"message": "Watch started",
		"watch":   h.watchManager.GetStatus(),
	})
}

// WatchRenew はWatchを更新
func (h *PubSubHandler) WatchRenew(c *gin.Context) {
	if h.watchManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "watchManager not initialized"})
		return
	}

	if err := h.watchManager.RenewWatch(c.Request.Context()); err != nil {
		log.Printf("Failed to renew watch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"message": "Watch renewed",
		"watch":   h.watchManager.GetStatus(),
	})
}

// WatchStop はWatch監視を停止
func (h *PubSubHandler) WatchStop(c *gin.Context) {
	if h.watchManager == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "watchManager not initialized"})
		return
	}

	if err := h.watchManager.StopWatch(c.Request.Context()); err != nil {
		log.Printf("Failed to stop watch: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "OK",
		"message": "Watch stopped",
	})
}

// WatchStatus はWatch状態を取得
func (h *PubSubHandler) WatchStatus(c *gin.Context) {
	if h.watchManager == nil {
		c.JSON(http.StatusOK, gin.H{
			"status": "OK",
			"watch":  map[string]interface{}{"active": false, "message": "watchManager not initialized"},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "OK",
		"watch":  h.watchManager.GetStatus(),
	})
}
