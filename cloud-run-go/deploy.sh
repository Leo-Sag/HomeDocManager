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
    --memory 512Mi \
    --cpu 1 \
    --timeout 540 \
    --concurrency 80 \
    --max-instances 10 \
    --set-env-vars "GCP_PROJECT_ID=${PROJECT_ID},GCP_REGION=${REGION},GCP_PROJECT_NUMBER=${PROJECT_NUMBER},WEBHOOK_URL=${WEBHOOK_URL}" \
    --service-account homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com

echo -e "${GREEN}デプロイ完了！${NC}"

# サービスURLを表示
SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region ${REGION} --project ${PROJECT_ID} --format 'value(status.url)')
echo -e "${GREEN}Service URL: ${SERVICE_URL}${NC}"
