#!/bin/bash
# Cleanup all old Watch channels

set -e

PROJECT_ID="family-document-manager-486009"
REGION="asia-northeast1"
SERVICE_NAME="homedocmanager-go"
SERVICE_URL="https://${SERVICE_NAME}-493569650708.${REGION}.run.app"

echo "=== Cleanup Old Watch Channels ==="

# Get ADMIN_TOKEN
ADMIN_TOKEN=$(gcloud secrets versions access latest --secret=ADMIN_TOKEN --project="${PROJECT_ID}")

# Stop current watch
echo "Stopping current watch..."
curl -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Length: 0" \
  "${SERVICE_URL}/admin/watch/stop" 2>&1 | grep -E "(status|message)" || true

# Wait a moment
sleep 2

# Start fresh watch
echo "Starting fresh watch..."
curl -X POST \
  -H "Authorization: Bearer ${ADMIN_TOKEN}" \
  -H "Content-Length: 0" \
  "${SERVICE_URL}/admin/watch/start" 2>&1 | grep -E "(status|message|channelId)" || true

echo ""
echo "Done. New watch started."
echo "Old watches will expire in 7 days (2026-02-16) if not stopped manually."
