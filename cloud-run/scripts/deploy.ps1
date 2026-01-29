# Cloud Runデプロイスクリプト (PowerShell)

$ErrorActionPreference = "Stop"

# 設定
$PROJECT_ID = "bright-lattice-328909"
$REGION = "asia-northeast1"
$SERVICE_NAME = "document-processor"

$OAUTH_CLIENT_ID = "333818874776-gt8evbue4jjbnmpjms9u7h67m3lgprc4.apps.googleusercontent.com"

Write-Host "=== Cloud Runデプロイスクリプト ==="
Write-Host "プロジェクトID: $PROJECT_ID"
Write-Host "リージョン: $REGION"
Write-Host "サービス名: $SERVICE_NAME"
Write-Host ""

# GCPプロジェクト設定
Write-Host "GCPプロジェクトを設定中..."
gcloud config set project $PROJECT_ID

# コンテナビルド
Write-Host "コンテナをビルド中..."
gcloud builds submit --tag gcr.io/${PROJECT_ID}/${SERVICE_NAME}

# Cloud Runデプロイ
Write-Host "Cloud Runにデプロイ中..."
gcloud run deploy $SERVICE_NAME `
  --image gcr.io/${PROJECT_ID}/${SERVICE_NAME} `
  --platform managed `
  --region $REGION `
  --no-allow-unauthenticated `
  --memory 2Gi `
  --timeout 300 `
  --set-env-vars "GCP_PROJECT_ID=${PROJECT_ID},GCP_REGION=${REGION},OAUTH_CLIENT_ID=${OAUTH_CLIENT_ID}" `
  --update-secrets GEMINI_API_KEY=GEMINI_API_KEY:latest, OAUTH_CLIENT_SECRET=oauth-client-secret:latest, PHOTOS_REFRESH_TOKEN=PHOTOS_REFRESH_TOKEN:latest

Write-Host ""
Write-Host "=== デプロイ完了 ==="
Write-Host "サービスURL:"
gcloud run services describe $SERVICE_NAME --region $REGION --format='value(status.url)'
