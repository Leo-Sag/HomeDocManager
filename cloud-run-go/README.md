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
    --memory 256Mi \
    --cpu 1 \
    --timeout 540 \
    --concurrency 80 \
    --max-instances 10 \
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
  -H "Content-Type: application/json" \
  -d '{"file_id": "YOUR_FILE_ID"}'
```

### GET /admin/info
ストレージ情報の取得

### POST /admin/cleanup
サービスアカウントのストレージクリーンアップ

### POST /trigger/inbox
Inboxフォルダ内の全ファイルを一括処理

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

以下の機能は現在スタブ実装となっており、今後実装予定です：

- [ ] Google Photos APIの完全実装
- [ ] Google Calendar APIの完全実装
- [ ] Google Tasks APIの完全実装
- [ ] NotebookLM同期機能の完全実装
- [ ] PDF処理の高度化（ページレンダリング）
- [ ] 構造化ログ（JSON形式）
- [ ] 分散トレーシング（OpenTelemetry）
- [ ] ユニットテスト・統合テスト

## ライセンス

Private Use Only
