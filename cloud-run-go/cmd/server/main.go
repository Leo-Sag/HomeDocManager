package main

import (
	"context"
	"fmt"
	"log"
	"os"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/gin-gonic/gin"
	"github.com/leo-sagawa/homedocmanager/internal/config"
	"github.com/leo-sagawa/homedocmanager/internal/handler"
	"github.com/leo-sagawa/homedocmanager/internal/linebot"
	"github.com/leo-sagawa/homedocmanager/internal/service"
)

func main() {
	ctx := context.Background()

	// サービスの初期化
	services, err := initServices(ctx)
	if err != nil {
		log.Fatalf("Failed to initialize services: %v", err)
	}

	// Webhook URLの設定
	webhookURL := getWebhookURL()
	log.Printf("Webhook URL: %s", webhookURL)

	// WatchManagerの初期化
	var watchManager *service.WatchManager
	if webhookURL != "" {
		watchManager = service.NewWatchManager(services.DriveClient, services.FileSorter, webhookURL)
		log.Printf("WatchManager initialized")
	} else {
		log.Printf("Warning: Webhook URL not configured, WatchManager disabled")
	}

	// Ginルーターの設定
	router := gin.Default()

	// ハンドラーの初期化
	pubsubHandler := handler.NewPubSubHandler(services, watchManager)

	// ルートの設定
	router.POST("/", pubsubHandler.HandlePubSub)
	router.GET("/health", pubsubHandler.HealthCheck)
	router.POST("/test", pubsubHandler.TestEndpoint)
	router.GET("/admin/info", pubsubHandler.AdminInfo)
	router.POST("/admin/cleanup", pubsubHandler.AdminCleanup)
	router.POST("/trigger/inbox", pubsubHandler.TriggerInbox)

	// Drive Watch関連のエンドポイント
	router.POST("/webhook/drive", pubsubHandler.HandleDriveWebhook)
	router.POST("/admin/watch/start", pubsubHandler.WatchStart)
	router.POST("/admin/watch/renew", pubsubHandler.WatchRenew)
	router.POST("/admin/watch/stop", pubsubHandler.WatchStop)
	router.GET("/admin/watch/status", pubsubHandler.WatchStatus)

	// LINE Bot Webhook
	if config.LineChannelSecret != "" && config.LineChannelAccessToken != "" {
		lineService, err := linebot.NewService(config.LineBotSettingsPath)
		if err != nil {
			log.Printf("Warning: LINE Bot Service initialization failed: %v", err)
		} else {
			// RAGService初期化（オプショナル）
			var ragService *linebot.RAGService
			geminiAPIKey := os.Getenv("GEMINI_API_KEY")
			if geminiAPIKey == "" {
				// Secret Managerから取得を試行
				geminiAPIKey = getSecretValue(ctx, config.SecretGeminiAPIKey)
			}
			if geminiAPIKey != "" {
				ragService, err = linebot.NewRAGService(ctx, geminiAPIKey, config.LineUserSettingsPath, services.DriveClient)
				if err != nil {
					log.Printf("Warning: RAG Service initialization failed: %v", err)
					ragService = nil
				} else if ragService != nil {
					log.Printf("RAG Service initialized with model: %s", config.GeminiModelsConfig.LineRAG)
				} else {
					log.Printf("Info: RAG Service disabled (no documents or folders configured)")
				}
			} else {
				log.Printf("Info: GEMINI_API_KEY not set, RAG Service disabled")
			}

			lineHandler, err := linebot.NewHandler(config.LineChannelSecret, config.LineChannelAccessToken, lineService, ragService)
			if err != nil {
				log.Printf("Warning: LINE Bot Handler initialization failed: %v", err)
			} else {
				router.POST("/callback", lineHandler.HandleWebhook)
				log.Printf("LINE Bot Webhook registered at /callback")
			}
		}
	} else {
		log.Printf("Info: LINE Bot credentials not set, /callback endpoint disabled")
	}

	// ポート設定
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on port %s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// getWebhookURL はCloud RunのサービスURLを取得してWebhook URLを生成
func getWebhookURL() string {
	// 環境変数から明示的に指定されている場合はそれを使用
	if url := os.Getenv("WEBHOOK_URL"); url != "" {
		return url
	}

	// Cloud Run環境では K_SERVICE と K_REVISION が設定される
	serviceName := os.Getenv("K_SERVICE")
	if serviceName == "" {
		return ""
	}

	// プロジェクトIDとリージョンから構築
	projectID := os.Getenv("GCP_PROJECT_ID")
	region := os.Getenv("GCP_REGION")
	if region == "" {
		region = "asia-northeast1"
	}

	// Cloud RunのデフォルトURL形式
	// 新形式: https://{service}-{project_number}.{region}.run.app
	// プロジェクト番号が必要なので、環境変数で明示するか、旧形式を使用
	projectNumber := os.Getenv("GCP_PROJECT_NUMBER")
	if projectNumber != "" {
		return fmt.Sprintf("https://%s-%s.%s.run.app/webhook/drive", serviceName, projectNumber, region)
	}

	// プロジェクト番号がない場合は旧形式を試行
	if projectID != "" {
		return fmt.Sprintf("https://%s-%s.a.run.app/webhook/drive", serviceName, projectID)
	}

	return ""
}

// initServices は全サービスを初期化
func initServices(ctx context.Context) (*service.Services, error) {
	// AIRouter
	aiRouter, err := service.NewAIRouter(ctx)
	if err != nil {
		return nil, err
	}

	// DriveClient
	driveClient, err := service.NewDriveClient(ctx)
	if err != nil {
		return nil, err
	}

	// PDFProcessor
	pdfProcessor := service.NewPDFProcessor()

	// PhotosClient (オプショナル)
	var photosClient *service.PhotosClient
	photosClient, err = service.NewPhotosClient(ctx)
	if err != nil {
		log.Printf("Warning: PhotosClient initialization failed: %v", err)
		photosClient = nil
	}

	// CalendarClient (オプショナル)
	var calendarClient *service.CalendarClient
	calendarClient, err = service.NewCalendarClient(ctx)
	if err != nil {
		log.Printf("Warning: CalendarClient initialization failed: %v", err)
		calendarClient = nil
	}

	// TasksClient (オプショナル)
	var tasksClient *service.TasksClient
	tasksClient, err = service.NewTasksClient(ctx)
	if err != nil {
		log.Printf("Warning: TasksClient initialization failed: %v", err)
		tasksClient = nil
	}

	// NotebookLMSync (オプショナル)
	var notebooklmSync *service.NotebookLMSync
	notebooklmSync, err = service.NewNotebookLMSync(ctx, driveClient)
	if err != nil {
		log.Printf("Warning: NotebookLMSync initialization failed: %v", err)
		notebooklmSync = nil
	}

	// GradeManager
	gradeManager := service.NewGradeManager()

	// FileSorter
	fileSorter := service.NewFileSorter(
		aiRouter,
		pdfProcessor,
		driveClient,
		photosClient,
		calendarClient,
		tasksClient,
		notebooklmSync,
		gradeManager,
	)

	return &service.Services{
		AIRouter:       aiRouter,
		PDFProcessor:   pdfProcessor,
		DriveClient:    driveClient,
		PhotosClient:   photosClient,
		CalendarClient: calendarClient,
		TasksClient:    tasksClient,
		NotebookLMSync: notebooklmSync,
		GradeManager:   gradeManager,
		FileSorter:     fileSorter,
	}, nil
}

// getSecretValue はSecret Managerからシークレット値を取得
func getSecretValue(ctx context.Context, secretName string) string {
	// Secret Managerクライアントを作成
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		log.Printf("Secret Manager client creation failed: %v", err)
		return ""
	}
	defer client.Close()

	// Secret Managerからキーを取得
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest",
		config.GCPProjectID,
		secretName)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		log.Printf("Secret Manager access failed for %s: %v", secretName, err)
		return ""
	}

	return string(result.Payload.Data)
}
