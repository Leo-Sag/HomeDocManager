# HomeDocManager Go版

PythonからGo言語へリファクタリングした HomeDocManager の実装です。

## 主な改善点

### パフォーマンス
- **コールドスタート短縮**: 数秒 → 数十ミリ秒
- **メモリ使用量削減**: 512MB → 128-256MB（約50-75%削減）
- **並行処理効率化**: Goroutineによる高効率なI/O多重化

### コスト削減
- **メモリ課金削減**: 小さいメモリクラス(256MB)での動作が可能
- **CPU効率向上**: ネイティブバイナリによる高速処理
- **アイドル時課金削減**: スケールto 0での高速起動

### 保守性
- **型安全性**: コンパイル時の型チェック
- **公式SDK**: GoogleのGemini API公式SDK (google-genai-go) を使用
- **構造化ログ**: 構造化されたログ出力（JSON形式対応可能）

## プロジェクト構造

```
cloud-run-go/
├── cmd/
│   └── server/
│       └── main.go              # アプリケーションエントリーポイント
├── internal/
│   ├── config/
│   │   └── settings.go          # 設定
│   ├── handler/
│   │   └── pubsub.go            # HTTPハンドラー
│   ├── service/
│   │   ├── ai_router.go         # Gemini API
│   │   ├── drive_client.go      # Google Drive API
│   │   ├── photos_client.go     # Google Photos API
│   │   ├── calendar_client.go   # Google Calendar API
│   │   ├── tasks_client.go      # Google Tasks API
│   │   ├── grade_manager.go     # 学年管理
│   │   ├── notebooklm_sync.go   # NotebookLM同期
│   │   ├── pdf_processor.go     # PDF処理
│   │   ├── file_sorter.go       # メイン処理ロジック
│   │   └── services.go          # サービスコンテナ
│   └── model/
│       └── types.go             # データ型定義
├── Dockerfile
├── deploy.sh
├── go.mod
└── README.md
```

## デプロイ手順

### 1. 前提条件

- Go 1.23以上
- Docker
- gcloud CLI
- GCPプロジェクトの設定完了
- 必要なAPIの有効化:
  - Google Drive API
  - Google Photos Library API
  - Google Calendar API
  - Google Tasks API
  - Secret Manager API
  - Gemini API

### 2. Secret Managerの設定

以下のシークレットをSecret Managerに登録してください：

```bash
# Gemini APIキー
gcloud secrets create GEMINI_API_KEY \
    --data-file=- <<< "your-gemini-api-key"

# OAuth Refresh Token (Photos/Calendar/Tasks共用)
gcloud secrets create OAUTH_REFRESH_TOKEN \
    --data-file=- <<< "your-oauth-refresh-token"
```

### 2.1 本番用トークンの準備（推奨）

管理系エンドポイント保護およびDrive Webhook検証用トークンを用意します。

```bash
# 32バイトのランダムトークンを生成
openssl rand -hex 32
```

生成したトークンは以下に設定します：

- `ADMIN_TOKEN`: 管理系エンドポイント用（`/admin/*`, `/test`, `/trigger/inbox`）
- `DRIVE_WEBHOOK_TOKEN`: Drive Watch通知検証用

### 2.2 Secret Managerにトークンを登録する場合（任意）

```bash
gcloud secrets create ADMIN_TOKEN --data-file=- <<< "your-admin-token"
gcloud secrets create DRIVE_WEBHOOK_TOKEN --data-file=- <<< "your-drive-webhook-token"
```

### 3. サービスアカウントの設定

```bash
# サービスアカウントを作成
gcloud iam service-accounts create homedocmanager-sa \
    --display-name="HomeDocManager Service Account"

# 必要な権限を付与
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
    --member="serviceAccount:homedocmanager-sa@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
    --role="roles/secretmanager.secretAccessor"

# Drive APIへのアクセス権限（必要に応じてDriveのフォルダに対する共有設定も行う）
```

### 4. 設定ファイルの編集

[internal/config/settings.go](internal/config/settings.go) を編集して、以下の項目を設定してください：

- `GCPProjectID`: あなたのGCPプロジェクトID
- `FolderIDs`: Google DriveのフォルダID（必要に応じて変更）
- その他の設定項目

### 5. デプロイ

```bash
# デプロイスクリプトを編集（PROJECT_IDを設定）
vi deploy.sh

# 本番用トークンと設定（推奨）
export ADMIN_AUTH_MODE=required
export ADMIN_TOKEN="your-admin-token"
export DRIVE_WEBHOOK_TOKEN="your-drive-webhook-token"

# 統合Gemini呼び出しを無効化する場合
# export ENABLE_COMBINED_GEMINI=false

# デプロイ実行
./deploy.sh
```

または、手動でデプロイ：

```bash
# Dockerイメージのビルド
docker build -t gcr.io/YOUR_PROJECT_ID/homedocmanager-go:latest .

# GCRにプッシュ
docker push gcr.io/YOUR_PROJECT_ID/homedocmanager-go:latest

# Cloud Runにデプロイ
gcloud run deploy homedocmanager-go \
    --image gcr.io/YOUR_PROJECT_ID/homedocmanager-go:latest \
    --platform managed \
    --region asia-northeast1 \
    --allow-unauthenticated \
    --memory 384Mi \
    --cpu 1 \
    --timeout 540 \
    --concurrency 4 \
    --max-instances 3 \
    --set-env-vars "GCP_PROJECT_ID=YOUR_PROJECT_ID,GCP_REGION=asia-northeast1,ADMIN_AUTH_MODE=required,ADMIN_TOKEN=your-admin-token,DRIVE_WEBHOOK_TOKEN=your-drive-webhook-token" \
    --service-account homedocmanager-sa@YOUR_PROJECT_ID.iam.gserviceaccount.com
```

Secret Managerを使う場合は `--set-secrets` を推奨します：

```bash
gcloud run deploy homedocmanager-go \
  --image gcr.io/YOUR_PROJECT_ID/homedocmanager-go:latest \
  --platform managed \
  --region asia-northeast1 \
  --allow-unauthenticated \
  --memory 384Mi \
  --cpu 1 \
  --timeout 540 \
  --concurrency 4 \
  --max-instances 3 \
  --set-env-vars "GCP_PROJECT_ID=YOUR_PROJECT_ID,GCP_REGION=asia-northeast1,ADMIN_AUTH_MODE=required" \
  --set-secrets "ADMIN_TOKEN=ADMIN_TOKEN:latest,DRIVE_WEBHOOK_TOKEN=DRIVE_WEBHOOK_TOKEN:latest" \
  --service-account homedocmanager-sa@YOUR_PROJECT_ID.iam.gserviceaccount.com
```

### 6. Pub/Subトリガーの設定

Python版と同じように、Google DriveのファイルイベントをPub/Subで受信する設定が必要です。

```bash
# Pub/Subトピックの作成（既存の場合はスキップ）
gcloud pubsub topics create drive-events

# Cloud Runサービスへのサブスクリプション作成
gcloud pubsub subscriptions create homedocmanager-go-sub \
    --topic=drive-events \
    --push-endpoint=https://YOUR_CLOUD_RUN_URL/ \
    --ack-deadline=600
```

## エンドポイント

### POST /
Pub/Subトリガーのメインエンドポイント

### GET /health
ヘルスチェック

### POST /test
テスト用エンドポイント（手動トリガー）

```bash
curl -X POST https://YOUR_CLOUD_RUN_URL/test \
  -H "Authorization: Bearer your-admin-token" \
  -H "Content-Type: application/json" \
  -d '{"file_id": "YOUR_FILE_ID"}'
```

### GET /admin/info
ストレージ情報の取得

### GET /admin/ping
管理系認証の疎通確認（副作用なし）

### POST /admin/cleanup
サービスアカウントのストレージクリーンアップ

### POST /trigger/inbox
Inboxフォルダ内の全ファイルを一括処理

### Drive Webhook の検証

Drive Watch通知は以下のトークンが一致する場合のみ受理されます。

- `DRIVE_WEBHOOK_TOKEN` が設定されている場合のみ検証
- Watch作成時に同じトークンが `channel.Token` として送信されます

トークンが不一致の場合、`403` を返します。

## ローカル開発

```bash
# 依存関係のインストール
go mod download

# ローカルで実行
export GEMINI_API_KEY="your-api-key"
export GCP_PROJECT_ID="your-project-id"
go run cmd/server/main.go
```

## モニタリング

Cloud Runのログは自動的にCloud Loggingに送信されます：

```bash
# ログの確認
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go" \
    --limit 50 \
    --format json
```

ログ形式は環境変数で切り替えできます：

- `LOG_FORMAT=json` でJSON（構造化）ログ
- `LOG_LEVEL=debug|info|warn|error`

## トラブルシューティング

### コールドスタート時のタイムアウト

- `--timeout` の値を増やす（最大540秒）
- `--cpu` の値を増やす（1 → 2）

### メモリ不足エラー

- `--memory` の値を増やす（256Mi → 512Mi）

### Gemini API エラー

- Secret ManagerにAPIキーが正しく登録されているか確認
- APIキーの権限を確認

## Python版からの移行手順

1. **新しいCloud Runサービスとして並行稼働**
   - Python版とGo版を同時に稼働させて動作を比較

2. **トラフィックの段階的移行**
   - 一部のファイルをGo版で処理してテスト
   - 問題がなければ徐々にトラフィックを移行

3. **Python版の廃止**
   - Go版が安定稼働したら、Python版のCloud Runサービスを削除

## 今後の実装予定

以下は現状に合わせて更新しました（「スタブ/未実装」ではなく、実装済みまたは改善余地ありの項目です）：

- [x] Google Photos API（画像アップロード、PDFはページレンダリングして複数ページアップロード）
- [x] Google Calendar API（イベント作成、重複チェック）
- [x] Google Tasks API（タスク作成、重複チェック）
- [x] NotebookLM同期（年度×カテゴリの累積Docに追記、同期済みマーカーで重複同期を抑止）
- [x] PDF処理（`pdftoppm` によるページレンダリング、ページ順は数値ソートで安定化）
- [x] 構造化ログ（`LOG_FORMAT=json` でJSONログ、HTTPアクセスログは構造化）
- [~] 分散トレーシング（OpenTelemetry）
- [x] ユニットテスト（認証/トークン期限/PDFページ順などの基本テスト）

OpenTelemetry は現状「トレースID抽出（Cloud Run `X-Cloud-Trace-Context` / W3C `traceparent`）とログ相関」まで実装しています。
Collector/Exporter を含むフル構成は、運用先の要件（OTLP, Cloud Trace 等）に合わせて追加する想定です。

## Rollout Walkthrough

本番反映とロールバック（Cloud RunのRevision運用）を含む手順書は以下にまとめています：

- `WALKTHROUGH.md`

## ライセンス

Private Use Only
