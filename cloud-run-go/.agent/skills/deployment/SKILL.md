---
name: homedocmanager-deployment
description: homedocmanager-go を Google Cloud Run にデプロイし、README更新とGitプッシュを実行する完全なワークフロー
---

# homedocmanager-go デプロイワークフロー

このスキルは、`homedocmanager-go` を Google Cloud Run にデプロイし、ドキュメントを最新化する完全なワークフローを提供します。

## 前提条件

- GCP プロジェクト作成済み
- OAuth 2.0 クライアント（デスクトップアプリ）作成済み
- Service Account `homedocmanager-sa@{PROJECT_ID}.iam.gserviceaccount.com` 作成済み
  - 権限: Secret Manager Secret Accessor, Cloud Run Invoker
- 必要なシークレットが Secret Manager に登録済み

## 必須シークレット一覧

以下のシークレットを Secret Manager に登録し、Service Account にアクセス権限を付与してください。

| シークレット名 | 説明 | 生成方法 |
|---------------|------|----------|
| `ADMIN_TOKEN` | 管理APIの認証用 | `openssl rand -hex 32` |
| `DRIVE_WEBHOOK_TOKEN` | Drive Webhook検証用 | `openssl rand -hex 32` |
| `GEMINI_API_KEY` | Gemini API キー | Google AI Studio |
| `OAUTH_REFRESH_TOKEN` | OAuth リフレッシュトークン | `go run tools/setup_oauth.go` |
| `LINE_CHANNEL_SECRET` | LINE Bot チャンネルシークレット（オプション） | LINE Developers |
| `LINE_CHANNEL_ACCESS_TOKEN` | LINE Bot アクセストークン（オプション） | LINE Developers |

**シークレット登録例**:

```bash
ADMIN_TOKEN="$(openssl rand -hex 32)"
DRIVE_WEBHOOK_TOKEN="$(openssl rand -hex 32)"

echo -n "$ADMIN_TOKEN" | gcloud secrets versions add ADMIN_TOKEN --data-file=-
echo -n "$DRIVE_WEBHOOK_TOKEN" | gcloud secrets versions add DRIVE_WEBHOOK_TOKEN --data-file=-
echo -n "YOUR_GEMINI_API_KEY" | gcloud secrets versions add GEMINI_API_KEY --data-file=-
echo -n "YOUR_REFRESH_TOKEN" | gcloud secrets versions add OAUTH_REFRESH_TOKEN --data-file=-
```

## デプロイ手順

### 1. ビルド確認

変更をコミットする前に、ビルドが成功することを確認します。

```bash
cd cloud-run-go
go build -o /dev/null ./cmd/server
```

### 2. デプロイ実行

**推奨**: Cloud Build を使用したデプロイ（Docker不要）

```bash
cd cloud-run-go
./deploy-cloudbuild.sh
```

**代替**: ローカル Docker ビルド

```bash
cd cloud-run-go
export USE_SECRET_MANAGER=1
export ADMIN_AUTH_MODE=required
export LOG_FORMAT=json
./deploy.sh
```

### 3. デプロイ後の確認

```bash
SERVICE_URL="$(gcloud run services describe homedocmanager-go \
  --region asia-northeast1 --format='value(status.url)')"

# ヘルスチェック
curl -sS "$SERVICE_URL/health"

# 管理認証確認
ADMIN_TOKEN="$(gcloud secrets versions access latest --secret=ADMIN_TOKEN)"
curl -i "$SERVICE_URL/admin/ping" -H "Authorization: Bearer $ADMIN_TOKEN"

# Watch 状態確認（自動起動されているはず）
curl -sS "$SERVICE_URL/admin/watch/status" -H "Authorization: Bearer $ADMIN_TOKEN"
```

### 4. README更新（重要）

**コード変更時は必ずREADMEも更新してください**。

以下のセクションを確認・更新します：
- **機能一覧**: 新機能の追加
- **技術スタック**: 使用技術の変更
- **環境変数**: 新しい環境変数の追加
- **トラブルシューティング**: 新しい問題の追加

### 5. Git コミット・プッシュ

```bash
cd ..  # プロジェクトルートに戻る
git add README.md cloud-run-go/
git commit -m "適切なコミットメッセージ

Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>"
git push origin feature/cloud-run-go-redeploy
```

## デプロイ設定詳細

現在のCloud Run設定:

| 項目 | 値 | 説明 |
|------|-----|------|
| メモリ | 384Mi | PDF処理とGemini API呼び出しに最適化 |
| CPU | 1 | リクエスト課金 |
| 同時実行数 | 4 | メモリ使用量の予測可能性を確保 |
| 最大インスタンス | 3 | コスト暴走防止 |
| 最小インスタンス | 0 | スケール to ゼロでコスト削減 |
| タイムアウト | 540s | 大量ファイル処理に対応 |

### 環境変数

deploy-cloudbuild.sh で自動設定される環境変数:

- `GCP_PROJECT_ID`: GCPプロジェクトID
- `GCP_REGION`: asia-northeast1
- `GCP_PROJECT_NUMBER`: プロジェクト番号
- `OAUTH_CLIENT_ID`: OAuth 2.0 クライアントID（デフォルト値あり）
- `OAUTH_CLIENT_SECRET`: OAuth 2.0 クライアントシークレット（デフォルト値あり）
- `ADMIN_AUTH_MODE`: required
- `LOG_FORMAT`: json
- `ENABLE_COMBINED_GEMINI`: true

### シークレット

Secret Manager から自動マウント:

- `ADMIN_TOKEN:latest`
- `DRIVE_WEBHOOK_TOKEN:latest`
- `OAUTH_REFRESH_TOKEN:latest`
- `GEMINI_API_KEY:latest`
- `LINE_CHANNEL_SECRET:latest` (存在する場合)
- `LINE_CHANNEL_ACCESS_TOKEN:latest` (存在する場合)

## Cloud Scheduler 設定

デプロイ後、以下のSchedulerジョブが必要です：

### Watch 自動更新（毎週月・木 12:00 JST）

```bash
gcloud scheduler jobs create http watch-renew-daily \
  --schedule="0 12 * * 1,4" \
  --time-zone="Asia/Tokyo" \
  --uri="${SERVICE_URL}/admin/watch/renew" \
  --http-method=POST \
  --headers="Content-Type=application/json,Authorization=Bearer ${ADMIN_TOKEN}" \
  --location=asia-northeast1 \
  --project="${PROJECT_ID}"
```

### Inbox 定期スキャン（毎時）

```bash
gcloud scheduler jobs create http inbox-trigger-hourly \
  --schedule="0 * * * *" \
  --time-zone="UTC" \
  --uri="${SERVICE_URL}/trigger/inbox" \
  --http-method=GET \
  --headers="Authorization=Bearer ${ADMIN_TOKEN}" \
  --location=asia-northeast1 \
  --project="${PROJECT_ID}"
```

## 主要機能（リビジョン00044以降）

### 並行処理の重複防止

- **ファイル処理済みマーカー**: Drive Propertiesに`file_processed=true`を設定し、複数インスタンスが同時に処理しても重複を防止
- **タスク重複チェック強化**: タイトル+期日の組み合わせで既存タスクとの重複を防止

### 自動運用

- **起動時Watch自動開始**: インスタンス起動時に自動的にDrive Watchを開始（コールドスタート復旧）
- **OAuth優先認証**: NotebookLM同期・Google Photosアップロードは必ずOAuthで実行（SA容量制限回避）

### NotebookLM 同期

- **除外ベース方式**: `50_写真・その他`と`40_子供・教育/03_記録・作品・成績`を除く全カテゴリが同期対象
- **OAuth必須**: Service Accountでは15GB容量制限があるため、OAuth必須

## トラブルシューティング

### デプロイが失敗する

**原因**: OAUTH_CLIENT_ID/SECRET が設定されていない

**解決策**:
1. `deploy-cloudbuild.sh` にデフォルト値が設定されているか確認
2. 環境変数として明示的に設定: `export OAUTH_CLIENT_ID=...`
3. 再デプロイ

### NotebookLM 同期が失敗する（storageQuotaExceeded）

**原因**: OAuth リフレッシュトークンが設定されておらず、SAで動作している

**解決策**:
1. `tools/setup_oauth.go` でリフレッシュトークンを再取得
2. Secret Manager に登録: `OAUTH_REFRESH_TOKEN`
3. Service Account に Secret アクセス権限を付与
4. 再デプロイ

### タスクが重複して作成される

**原因**: 古いリビジョン（00043以前）を使用している

**解決策**:
- リビジョン 00044 以降にアップデート
- ログで「既に処理済みのファイルです」が表示されることを確認
- 既存の重複タスクは手動削除が必要

### LINE Bot が反応しない

**原因**: LINE シークレットが登録されていない

**解決策**:
1. `LINE_CHANNEL_SECRET` と `LINE_CHANNEL_ACCESS_TOKEN` を Secret Manager に登録
2. Service Account にアクセス権限を付与
3. 再デプロイ
4. ログで `LINE Bot Webhook registered at /callback` を確認

## チェックリスト

デプロイ時に以下を確認してください：

- [ ] コードがビルドできることを確認済み
- [ ] 変更内容をテスト済み
- [ ] **README.md を最新の内容に更新済み**
- [ ] コミットメッセージが明確で説明的
- [ ] 必要なシークレットが全てSecret Managerに登録済み
- [ ] デプロイ後のヘルスチェック完了
- [ ] Watch 状態が active であることを確認
- [ ] LINE Bot使用時は `/callback` エンドポイントが登録されていることを確認

## 参考情報

- デプロイスクリプト: `deploy-cloudbuild.sh` (推奨), `deploy.sh`
- 詳細手順書: `WALKTHROUGH.md`
- プロジェクトREADME: `../README.md`
- 設定ファイル: `internal/config/settings.go`
