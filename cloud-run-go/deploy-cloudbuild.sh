#!/bin/bash

# Cloud Run デプロイスクリプト (Cloud Build版)
# ローカルにDockerがなくてもデプロイ可能

set -e

# 設定
PROJECT_ID="family-document-manager-486009"
PROJECT_NUMBER="493569650708"
REGION="asia-northeast1"
SERVICE_NAME="homedocmanager-go"
IMAGE_NAME="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"
WEBHOOK_URL="https://${SERVICE_NAME}-${PROJECT_NUMBER}.${REGION}.run.app/webhook/drive"
ENV_VARS="GCP_PROJECT_ID=${PROJECT_ID},GCP_REGION=${REGION},GCP_PROJECT_NUMBER=${PROJECT_NUMBER},WEBHOOK_URL=${WEBHOOK_URL}"

# Optional env vars (set in shell before running)
ADMIN_AUTH_MODE="${ADMIN_AUTH_MODE:-required}"
LOG_FORMAT="${LOG_FORMAT:-json}"
LOG_LEVEL="${LOG_LEVEL:-info}"
ENABLE_COMBINED_GEMINI="${ENABLE_COMBINED_GEMINI:-true}"

ENV_VARS="${ENV_VARS},ADMIN_AUTH_MODE=${ADMIN_AUTH_MODE},LOG_FORMAT=${LOG_FORMAT},LOG_LEVEL=${LOG_LEVEL},ENABLE_COMBINED_GEMINI=${ENABLE_COMBINED_GEMINI}"

if [ -n "${OAUTH_CLIENT_ID:-}" ]; then
    ENV_VARS="${ENV_VARS},OAUTH_CLIENT_ID=${OAUTH_CLIENT_ID}"
fi
if [ -n "${OAUTH_CLIENT_SECRET:-}" ]; then
    ENV_VARS="${ENV_VARS},OAUTH_CLIENT_SECRET=${OAUTH_CLIENT_SECRET}"
fi

SECRETS="ADMIN_TOKEN=ADMIN_TOKEN:latest,DRIVE_WEBHOOK_TOKEN=DRIVE_WEBHOOK_TOKEN:latest,GEMINI_API_KEY=GEMINI_API_KEY:latest"
if gcloud secrets describe OAUTH_REFRESH_TOKEN --project "${PROJECT_ID}" >/dev/null 2>&1; then
    SECRETS="${SECRETS},OAUTH_REFRESH_TOKEN=OAUTH_REFRESH_TOKEN:latest"
else
    echo "Warning: OAUTH_REFRESH_TOKEN secret not found; skipping injection"
fi

echo "=== HomeDocManager Go版デプロイ (Cloud Build) ==="
echo "Project ID: ${PROJECT_ID}"
echo "Image: ${IMAGE_NAME}"

# Cloud Buildでビルド＆プッシュ
echo "Cloud Buildでビルド中..."
gcloud builds submit --tag ${IMAGE_NAME}:latest --project ${PROJECT_ID}

# Cloud Runにデプロイ
echo "Cloud Runにデプロイ中..."
gcloud run deploy ${SERVICE_NAME} \
    --image ${IMAGE_NAME}:latest \
    --platform managed \
    --region ${REGION} \
    --project ${PROJECT_ID} \
    --allow-unauthenticated \
    --memory 384Mi \
    --cpu 1 \
    --timeout 540 \
    --concurrency 4 \
    --max-instances 3 \
    --set-env-vars "${ENV_VARS}" \
    --set-secrets "${SECRETS}" \
    --service-account homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com

echo "デプロイ完了！"

# サービスURLを表示
SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region ${REGION} --project ${PROJECT_ID} --format 'value(status.url)')
echo "Service URL: ${SERVICE_URL}"
