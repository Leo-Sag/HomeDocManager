#!/bin/bash
# Cloud Runデプロイスクリプト

set -e

# 設定
PROJECT_ID="${GCP_PROJECT_ID:-your-project-id}"
REGION="${GCP_REGION:-asia-northeast1}"
SERVICE_NAME="document-processor"

echo "=== Cloud Runデプロイスクリプト ==="
echo "プロジェクトID: $PROJECT_ID"
echo "リージョン: $REGION"
echo "サービス名: $SERVICE_NAME"
echo ""

# GCPプロジェクト設定
echo "GCPプロジェクトを設定中..."
gcloud config set project $PROJECT_ID

# コンテナビルド
echo "コンテナをビルド中..."
gcloud builds submit --tag gcr.io/${PROJECT_ID}/${SERVICE_NAME}

# Cloud Runデプロイ
echo "Cloud Runにデプロイ中..."
gcloud run deploy ${SERVICE_NAME} \
  --image gcr.io/${PROJECT_ID}/${SERVICE_NAME} \
  --platform managed \
  --region ${REGION} \
  --no-allow-unauthenticated \
  --memory 1Gi \
  --timeout 300 \
  --set-env-vars GCP_PROJECT_ID=${PROJECT_ID},GCP_REGION=${REGION}

echo ""
echo "=== デプロイ完了 ==="
echo "サービスURL:"
gcloud run services describe ${SERVICE_NAME} --region ${REGION} --format='value(status.url)'
