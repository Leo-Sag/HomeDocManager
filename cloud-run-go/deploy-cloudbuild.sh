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
    --memory 512Mi \
    --cpu 1 \
    --timeout 540 \
    --concurrency 80 \
    --max-instances 10 \
    --set-env-vars "GCP_PROJECT_ID=${PROJECT_ID},GCP_REGION=${REGION},GCP_PROJECT_NUMBER=${PROJECT_NUMBER},WEBHOOK_URL=${WEBHOOK_URL},OAUTH_CLIENT_ID=493569650708-k1m3knr8em27foe6h13fotfknhuqpsss.apps.googleusercontent.com,OAUTH_CLIENT_SECRET=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" \
    --service-account homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com

echo "デプロイ完了！"

# サービスURLを表示
SERVICE_URL=$(gcloud run services describe ${SERVICE_NAME} --region ${REGION} --project ${PROJECT_ID} --format 'value(status.url)')
echo "Service URL: ${SERVICE_URL}"
