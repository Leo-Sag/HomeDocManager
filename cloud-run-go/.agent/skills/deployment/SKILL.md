---
name: homedocmanager-deployment
description: homedocmanager-go を Google Cloud Run にデプロイするための完全な手順。シークレット管理、ビルド、デプロイフラグの詳細を含む。
---

# homedocmanager-go デプロイガイド

このスキルは、`homedocmanager-go` を Google Cloud Run に正確かつセキュアにデプロイするための手順を提供します。

## 1. 準備 (Preparation)

### 1-1. 生成するシークレット

以下のトークンを生成し、Secret Manager に登録する必要があります。

- `ADMIN_TOKEN`: 管理 API の認証用。
- `DRIVE_WEBHOOK_TOKEN`: Google Drive Webhook の検証用。

**トークン生成方法 (PowerShell):**

```powershell
$ADMIN_TOKEN = -join ((48..57) + (97..102) | Get-Random -Count 64 | % {[char]$_})
$DRIVE_WEBHOOK_TOKEN = -join ((48..57) + (97..102) | Get-Random -Count 64 | % {[char]$_})
```

### 1-2. Secret Manager への登録 (Python スクリプト等の活用)

`register_secrets.py` 等を使用して、以下のシークレットを `latest` バージョンとして登録します。

- `ADMIN_TOKEN`
- `DRIVE_WEBHOOK_TOKEN`
- `OAUTH_REFRESH_TOKEN` (既存のものがあれば流用)
- `GEMINI_API_KEY` (既存のものがあれば流用)

## 2. ビルド (Build)

Cloud Build を使用して Docker イメージをビルドし、GCR にプッシュします。

```powershell
gcloud builds submit --tag gcr.io/[PROJECT_ID]/homedocmanager-go:latest --project [PROJECT_ID]
```

## 3. デプロイ (Deploy)

正確な環境変数を設定して Cloud Run サービスをデプロイします。**特に `OAUTH_CLIENT_ID` と `OAUTH_CLIENT_SECRET` の指定を忘れないようにしてください。**

```powershell
gcloud run deploy homedocmanager-go `
  --image gcr.io/[PROJECT_ID]/homedocmanager-go:latest `
  --platform managed `
  --region [REGION] `
  --allow-unauthenticated `
  --memory 384Mi `
  --cpu 1 `
  --timeout 540 `
  --concurrency 4 `
  --max-instances 3 `
  --set-env-vars "GCP_PROJECT_ID=[PROJECT_ID],GCP_REGION=[REGION],ADMIN_AUTH_MODE=required,OAUTH_CLIENT_ID=[CLIENT_ID],OAUTH_CLIENT_SECRET=[CLIENT_SECRET],WEBHOOK_URL=https://homedocmanager-go-[PROJECT_NUMBER].[REGION].run.app/webhook/drive" `
  --set-secrets "ADMIN_TOKEN=ADMIN_TOKEN:latest,DRIVE_WEBHOOK_TOKEN=DRIVE_WEBHOOK_TOKEN:latest,OAUTH_REFRESH_TOKEN=OAUTH_REFRESH_TOKEN:latest,GEMINI_API_KEY=GEMINI_API_KEY:latest" `
  --service-account [SERVICE_ACCOUNT_EMAIL] `
  --project [PROJECT_ID]
```

## 4. 検証 (Verification)

デプロイ後、以下の順序で動作を確認します。

1. **ヘルスチェック**: `GET /health` ("OK" が返ることを確認)
2. **管理情報取得**: `GET /admin/info` (`Authorization: Bearer [ADMIN_TOKEN]` が必要)
3. **ステータス確認**: `GET /admin/watch/status` (自動で `active` になっていることを確認)

## トラブルシューティング

- **応答が非常に遅い/タイムアウトする**: 初回起動時の OAuth 認証情報の読み込みや、Drive API への初回アクセスで時間がかかる場合があります。`ADMIN_TOKEN` が正しいことを確認の上、再度リバインド（再起動）を試みてください。
- **401 Unauthorized**: Secret Manager の `ADMIN_TOKEN` とリクエストヘッダーのトークンが一致しているか、環境変数が正しく反映されているかを確認してください。
