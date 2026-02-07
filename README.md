# HomeDocManager

Google Drive 上の家庭内書類（PDF・画像）を Gemini AI で自動解析し、カテゴリ分類・リネーム・フォルダ振り分けを行う Cloud Run マイクロサービスです。
Google Calendar / Tasks / Photos / NotebookLM との連携、LINE Bot によるドキュメント検索（RAG）にも対応しています。

## 機能一覧

### ファイル自動仕分け

- Inbox（`00_Inbox`）フォルダに入ったファイルを検知し、Gemini Flash/Pro で内容を解析
- 解析結果に基づき `YYYYMMDD_要約.ext` 形式にリネームし、カテゴリ別・年度別フォルダへ自動移動
- 子供の名前・学年・クラス名を OCR から自動特定し、子供ごとのサブフォルダに振り分け
- 統合 Gemini 呼び出し（`ENABLE_COMBINED_GEMINI`）により、分類・予定抽出・OCR を 1 回の API 呼び出しで実行可能

### カレンダー・タスク連携

- 学校のお便り等から行事予定を抽出し Google Calendar に登録
- 提出期限等を Google Tasks に登録（同一日のタスクは自動マージ）
- 重複チェックにより既存の予定・タスクとの二重登録を防止

### NotebookLM 同期

- 処理済みドキュメントの OCR テキスト・事実・要約を年度別・カテゴリ別の Google Docs に追記
- 対象カテゴリ: マネー・税務 / プロジェクト・資産 / ライフ・行政 / 子供・教育 / ヘルス・医療 / ライブラリ
- ファイルプロパティによる同期済みマーカーで重複同期を防止

### Google Photos 連携

- 写真カテゴリおよび子供の記録・作品カテゴリのファイルを Google Photos に自動アップロード
- PDF は 300 DPI で画像変換してアップロード（ページ順は数値ソートで安定化）

### LINE Bot（RAG 対応）

- 蓄積ドキュメントに対する自然言語 Q&A（RAG）
- 回答のソース元 Google Drive URL を自動提示
- カテゴリ別ナビゲーション（Flex Message）・クイックリプライ対応

### 管理・運用

- 管理系エンドポイントはトークン認証で保護（`ADMIN_TOKEN`）
- Drive Watch webhook のトークン検証（`DRIVE_WEBHOOK_TOKEN`）
- 構造化ログ（`slog` ベース、Cloud Logging 互換 severity / trace 相関）

## アーキテクチャ

```
Google Drive (Inbox)
    |
    +-- Pub/Sub push ----> POST /
    +-- Drive Watch -----> POST /webhook/drive
                              |
                      +-------v--------+
                      |  Cloud Run     |
                      |  (Go / Gin)    |
                      |                |
                      |  FileSorter    |
                      |   +- AIRouter  |--> Gemini Flash / Pro
                      |   +- Drive     |--> Google Drive API
                      |   +- Photos    |--> Google Photos API
                      |   +- Calendar  |--> Google Calendar API
                      |   +- Tasks     |--> Google Tasks API
                      |   +- Notebook  |--> Google Docs API
                      |                |
                      |  LINE Bot      |
                      |   +- RAG      -|--> Gemini (RAG)
                      +----------------+
```

## ディレクトリ構造

```
HomeDocManager/
+-- cloud-run-go/                  # アプリケーション本体
|   +-- cmd/server/main.go         # エントリポイント
|   +-- internal/
|   |   +-- config/settings.go     # 設定定数・環境変数
|   |   +-- handler/
|   |   |   +-- pubsub.go          # HTTP ハンドラー
|   |   |   +-- admin_auth.go      # 管理認証ミドルウェア
|   |   +-- service/
|   |   |   +-- ai_router.go       # Gemini Flash/Pro ルーティング
|   |   |   +-- file_sorter.go     # ファイル仕分けオーケストレータ
|   |   |   +-- drive_client.go    # Google Drive API クライアント
|   |   |   +-- photos_client.go   # Google Photos API クライアント
|   |   |   +-- calendar_client.go # Google Calendar API クライアント
|   |   |   +-- tasks_client.go    # Google Tasks API クライアント
|   |   |   +-- notebooklm_sync.go # NotebookLM 同期
|   |   |   +-- pdf_processor.go   # PDF -> 画像変換 (poppler)
|   |   |   +-- watch_manager.go   # Drive Watch 管理
|   |   |   +-- grade_manager.go   # 学年・クラス管理
|   |   |   +-- auth_helper.go     # OAuth 認証ヘルパー
|   |   |   +-- services.go        # サービスコンテナ
|   |   +-- linebot/
|   |   |   +-- handler.go         # LINE webhook ハンドラー
|   |   |   +-- service.go         # Flex Message テンプレート
|   |   |   +-- rag_service.go     # RAG 検索
|   |   +-- model/types.go         # データ型定義
|   |   +-- observability/
|   |       +-- init.go            # 構造化ログ初期化
|   |       +-- gin.go             # アクセスログ・リクエスト ID
|   |       +-- trace.go           # Cloud Trace 相関
|   +-- resources/linebot/         # LINE Bot テンプレート JSON
|   +-- tools/                     # セットアップツール
|   +-- Dockerfile                 # マルチステージビルド
|   +-- deploy.sh                  # ローカル Docker ビルド + デプロイ
|   +-- deploy-cloudbuild.sh       # Cloud Build デプロイ
|   +-- WALKTHROUGH.md             # 本番反映手順書
|   +-- go.mod
|   +-- go.sum
+-- _archive/                      # 旧実装 (Python / GAS)
+-- README.md                      # 本ファイル
```

## 技術スタック

| 項目 | 技術 |
|------|------|
| 言語 | Go 1.24 |
| Web フレームワーク | Gin |
| AI | Gemini 3 Flash / Pro (google/generative-ai-go) |
| Google APIs | Drive v3, Docs v1, Calendar v3, Tasks v1, Photos (OAuth REST) |
| 認証 | OAuth 2.0 + Service Account フォールバック |
| シークレット管理 | Google Secret Manager |
| メッセージング | LINE Bot SDK v7 |
| コンテナ | Docker (マルチステージ Alpine) |
| PDF 処理 | poppler-utils (pdftoppm) |
| ログ | log/slog (Cloud Logging 互換 JSON) |

## API エンドポイント

| メソッド | パス | 認証 | 説明 |
|---------|------|------|------|
| `POST` | `/` | なし | Pub/Sub push トリガー |
| `GET` | `/health` | なし | ヘルスチェック |
| `POST` | `/webhook/drive` | webhook token | Drive Watch コールバック |
| `POST` | `/callback` | LINE 署名検証 | LINE Bot webhook |
| `POST` | `/test` | ADMIN_TOKEN | 手動ファイル処理テスト |
| `GET` | `/admin/ping` | ADMIN_TOKEN | 認証確認用 |
| `GET` | `/admin/info` | ADMIN_TOKEN | ストレージ情報取得 |
| `POST` | `/admin/cleanup` | ADMIN_TOKEN | SA ストレージクリーンアップ |
| `POST` | `/trigger/inbox` | ADMIN_TOKEN | Inbox 一括処理 |
| `POST` | `/admin/watch/start` | ADMIN_TOKEN | Drive Watch 開始 |
| `POST` | `/admin/watch/renew` | ADMIN_TOKEN | Drive Watch 更新 |
| `POST` | `/admin/watch/stop` | ADMIN_TOKEN | Drive Watch 停止 |
| `GET` | `/admin/watch/status` | ADMIN_TOKEN | Drive Watch 状態確認 |

## 環境変数

### 必須

| 変数 | 説明 |
|------|------|
| `GCP_PROJECT_ID` | GCP プロジェクト ID |
| `GCP_REGION` | Cloud Run リージョン (デフォルト: `asia-northeast1`) |
| `GCP_PROJECT_NUMBER` | プロジェクト番号 (webhook URL 組み立てに使用) |
| `OAUTH_CLIENT_ID` | OAuth 2.0 クライアント ID |
| `OAUTH_CLIENT_SECRET` | OAuth 2.0 クライアントシークレット |

### シークレット (Secret Manager または環境変数)

| 変数 | 説明 |
|------|------|
| `GEMINI_API_KEY` | Gemini API キー |
| `OAUTH_REFRESH_TOKEN` | OAuth リフレッシュトークン (Drive / Photos / Calendar / Tasks) |
| `ADMIN_TOKEN` | 管理エンドポイント認証トークン |
| `DRIVE_WEBHOOK_TOKEN` | Drive Watch webhook 検証トークン |

### オプション

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `ADMIN_AUTH_MODE` | `required` | 管理認証モード (`required` / `optional` / `disabled`) |
| `ENABLE_COMBINED_GEMINI` | `true` | 統合 Gemini 呼び出しの有効化 |
| `LOG_FORMAT` | `text` | ログ形式 (`json` で Cloud Logging 互換 JSON) |
| `LOG_LEVEL` | `info` | ログレベル (`debug` / `info` / `warn` / `error`) |
| `WEBHOOK_URL` | 自動生成 | Drive Watch webhook URL の明示指定 |
| `LINE_CHANNEL_SECRET` | - | LINE Bot チャンネルシークレット |
| `LINE_CHANNEL_ACCESS_TOKEN` | - | LINE Bot チャンネルアクセストークン |
| `PORT` | `8080` | サーバーポート |

## Cloud Run デプロイ設定

| 項目 | 値 |
|------|-----|
| メモリ | 384Mi |
| CPU | 1 |
| 同時実行数 | 4 |
| 最大インスタンス | 3 |
| タイムアウト | 540s |
| リージョン | asia-northeast1 |

## セットアップ

### 1. OAuth 認証情報の取得

```bash
cd cloud-run-go
go run tools/setup_oauth.go
```

取得したリフレッシュトークンを Secret Manager に登録します。

### 2. Secret Manager 登録

```bash
ADMIN_TOKEN="$(openssl rand -hex 32)"
DRIVE_WEBHOOK_TOKEN="$(openssl rand -hex 32)"

echo -n "$ADMIN_TOKEN" | gcloud secrets versions add ADMIN_TOKEN --data-file=-
echo -n "$DRIVE_WEBHOOK_TOKEN" | gcloud secrets versions add DRIVE_WEBHOOK_TOKEN --data-file=-
echo -n "YOUR_GEMINI_API_KEY" | gcloud secrets versions add GEMINI_API_KEY --data-file=-
echo -n "YOUR_REFRESH_TOKEN" | gcloud secrets versions add OAUTH_REFRESH_TOKEN --data-file=-
```

### 3. デプロイ

**ローカル Docker ビルド:**

```bash
cd cloud-run-go
export USE_SECRET_MANAGER=1
export ADMIN_AUTH_MODE=required
export LOG_FORMAT=json
./deploy.sh
```

**Cloud Build (Docker 不要):**

```bash
cd cloud-run-go
./deploy-cloudbuild.sh
```

詳細な手順は [WALKTHROUGH.md](cloud-run-go/WALKTHROUGH.md) を参照してください。

### 4. デプロイ後の確認

```bash
SERVICE_URL="$(gcloud run services describe homedocmanager-go \
  --region asia-northeast1 --format='value(status.url)')"

# ヘルスチェック
curl -sS "$SERVICE_URL/health"

# 管理認証の確認
curl -i "$SERVICE_URL/admin/ping" -H "Authorization: Bearer $ADMIN_TOKEN"
```

## テスト

```bash
cd cloud-run-go
go test ./...
```

## ライセンス

Private Use Only
