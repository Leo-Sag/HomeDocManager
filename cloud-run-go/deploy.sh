#!/bin/bash

# Cloud Run デプロイスクリプト (Go版)

set -e

# 設定
PROJECT_ID="family-document-manager-486009"
PROJECT_NUMBER="493569650708"
REGION="asia-northeast1"
SERVICE_NAME="homedocmanager-go"
IMAGE_NAME="gcr.io/${PROJECT_ID}/${SERVICE_NAME}"
WEBHOOK_URL="https://${SERVICE_NAME}-${PROJECT_NUMBER}.${REGION}.run.app/webhook/drive"
ENV_VARS="GCP_PROJECT_ID=${PROJECT_ID},GCP_REGION=${REGION},GCP_PROJECT_NUMBER=${PROJECT_NUMBER},WEBHOOK_URL=${WEBHOOK_URL}"
USE_SECRET_MANAGER="${USE_SECRET_MANAGER:-}"

# Optional env vars (set in shell before running)
if [ -n "${ADMIN_AUTH_MODE:-}" ]; then
    ENV_VARS="${ENV_VARS},ADMIN_AUTH_MODE=${ADMIN_AUTH_MODE}"
fi
if [ -z "${USE_SECRET_MANAGER}" ]; then
    if [ -n "${ADMIN_TOKEN:-}" ]; then
        ENV_VARS="${ENV_VARS},ADMIN_TOKEN=${ADMIN_TOKEN}"
    fi
    if [ -n "${DRIVE_WEBHOOK_TOKEN:-}" ]; then
        ENV_VARS="${ENV_VARS},DRIVE_WEBHOOK_TOKEN=${DRIVE_WEBHOOK_TOKEN}"
    fi
fi
if [ -n "${ENABLE_COMBINED_GEMINI:-}" ]; then
    ENV_VARS="${ENV_VARS},ENABLE_COMBINED_GEMINI=${ENABLE_COMBINED_GEMINI}"
fi
if [ -n "${LOG_FORMAT:-}" ]; then
    ENV_VARS="${ENV_VARS},LOG_FORMAT=${LOG_FORMAT}"
fi
if [ -n "${LOG_LEVEL:-}" ]; then
    ENV_VARS="${ENV_VARS},LOG_LEVEL=${LOG_LEVEL}"
fi
if [ -n "${OAUTH_CLIENT_ID:-}" ]; then
    ENV_VARS="${ENV_VARS},OAUTH_CLIENT_ID=${OAUTH_CLIENT_ID}"
fi
if [ -n "${OAUTH_CLIENT_SECRET:-}" ]; then
    ENV_VARS="${ENV_VARS},OAUTH_CLIENT_SECRET=${OAUTH_CLIENT_SECRET}"
fi

SET_SECRETS_ARGS=()
if [ -n "${USE_SECRET_MANAGER}" ]; then
    SECRETS="ADMIN_TOKEN=ADMIN_TOKEN:latest,DRIVE_WEBHOOK_TOKEN=DRIVE_WEBHOOK_TOKEN:latest,GEMINI_API_KEY=GEMINI_API_KEY:latest"

    # Optional: only add if the secret exists (avoids deploy failure for unused features).
    if gcloud secrets describe OAUTH_REFRESH_TOKEN --project "${PROJECT_ID}" >/dev/null 2>&1; then
        SECRETS="${SECRETS},OAUTH_REFRESH_TOKEN=OAUTH_REFRESH_TOKEN:latest"
    else
        echo -e "${YELLOW}Warning: OAUTH_REFRESH_TOKEN secret not found; skipping injection${NC}"
    fi

    SET_SECRETS_ARGS=(--set-secrets "${SECRETS}")
fi

# カラー出力用
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== HomeDocManager Go版デプロイ ===${NC}"

# Auth workaround
if [ -f "access_token.txt" ]; then
    echo "Using access_token.txt for gcloud auth"
    export CLOUDSDK_AUTH_ACCESS_TOKEN=$(cat access_token.txt | tr -d '[:space:]')
fi

# プロジェクトIDの確認
echo -e "${YELLOW}Project ID: ${PROJECT_ID}${NC}"
read -p "このプロジェクトIDで正しいですか? (y/n): " confirm
if [ "$confirm" != "y" ]; then
    echo -e "${RED}デプロイを中止しました${NC}"
    exit 1
fi

# Dockerイメージのビルド
echo -e "${GREEN}Dockerイメージをビルド中...${NC}"
docker build -t ${IMAGE_NAME}:latest .

# GCRにプッシュ
echo -e "${GREEN}GCRにプッシュ中...${NC}"
docker push ${IMAGE_NAME}:latest

# Cloud Runにデプロイ
echo -e "${GREEN}Cloud Runにデプロイ中...${NC}"
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
    "${SET_SECRETS_ARGS[@]}" \
    --service-account homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com

echo -e "${GREEN}デプロイ完了！${NC}"

# サービスURLを表示
SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region ${REGION} --project ${PROJECT_ID} --format 'value(status.url)')
echo -e "${GREEN}Service URL: ${SERVICE_URL}${NC}"
