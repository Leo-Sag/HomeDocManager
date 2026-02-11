# HomeDocManager

Google Drive 上の家庭内書類（PDF・画像）を Gemini AI で自動解析し、カテゴリ分類・リネーム・フォルダ振り分けを行う Cloud Run マイクロサービスです。
Google Calendar / Tasks / Photos / NotebookLM との連携、LINE Bot によるドキュメント検索（RAG）にも対応しています。

## ステータス

**本番環境デプロイ済み** (2026-02-09)
- OAuth 2.0 同意画面: 本番公開済み（Sensitive スコープのみ、Restricted スコープなし）
- OAuth スコープ: `drive.file`, `documents`, `calendar.events`, `tasks`, `photoslibrary.appendonly`
- Google 検証: 審査中（デモ動画提出済み: https://youtu.be/PZclODCN8vI）
- Cloud Run リビジョン: 00052-jlq（重複処理完全解消版）

**ポリシーページ**:
- プライバシーポリシー: https://leo-sag.github.io/HomeDocManager/privacy-policy.html
- 利用規約: https://leo-sag.github.io/HomeDocManager/terms-of-service.html

## 機能一覧

### ファイル自動仕分け

- Inbox（`00_Inbox`）フォルダに入ったファイルを検知し、Gemini Flash/Pro で内容を解析
- 解析結果に基づき `YYYYMMDD_要約.ext` 形式にリネームし、カテゴリ別・年度別フォルダへ自動移動
- 子供の名前・学年・クラス名を OCR から自動特定し、子供ごとのサブフォルダに振り分け
- 統合 Gemini 呼び出し（`ENABLE_COMBINED_GEMINI`）により、分類・予定抽出・OCR を 1 回の API 呼び出しで実行可能

### カレンダー・タスク連携

- 学校のお便り等から行事予定を抽出し Google Calendar に登録
- 提出期限等を Google Tasks に登録（同一日のタスクは自動マージ）
- **重複チェック強化**: タイトル+期日の組み合わせで既存タスクとの二重登録を防止
- **並行処理対応**: 複数インスタンスが同時に処理しても、タスクは1つのみ作成

### NotebookLM 同期

- 処理済みドキュメントの OCR テキスト・事実・要約を年度別・カテゴリ別の Google Docs に追記
- **除外ベース方式**: 以下を除くすべてのカテゴリが同期対象
  - `50_写真・その他` - 画像主体のため転記不可
  - `40_子供・教育/03_記録・作品・成績` - 画像・作品のため転記不可
- NotebookLM カテゴリマッピング: life / money / children / medical / library / assets
- OAuth 認証によりユーザーアカウント配下に統合ドキュメントを作成（SA 容量制限回避）
- ファイルプロパティによる同期済みマーカーで重複同期を防止

### Google Photos 連携

- 写真カテゴリおよび子供の記録・作品カテゴリのファイルを Google Photos に自動アップロード
- PDF は 300 DPI で画像変換してアップロード（ページ順は数値ソートで安定化）

### LINE Bot（RAG 対応）

- NotebookLM 同期済みドキュメントに対する自然言語 Q&A（RAG）
- Gemini Flash によるベクトル検索・意味理解ベースの回答生成
- 回答のソース元 Google Drive URL を自動提示
- カテゴリ別ナビゲーション（Flex Message）・クイックリプライ対応
- 家族メンバーごとのアクセス制御（大人情報 vs 子供情報の権限管理）

### Discord 通知

- ファイル処理エラー発生時に Discord Webhook で即時通知
- 毎時 Inbox スキャン完了時に処理結果サマリーを通知（処理 or エラーがある場合のみ）
- オプショナル機能（`DISCORD_WEBHOOK_URL` 未設定時は無効）

### 管理・運用

- 管理系エンドポイントはトークン認証で保護（`ADMIN_TOKEN`）
- Drive Watch webhook のトークン検証（`DRIVE_WEBHOOK_TOKEN`）
- **起動時 Watch 自動開始**: インスタンス起動時に自動的に Drive Watch を開始（コールドスタート復旧）
- **Cloud Scheduler 自動運用**:
  - `watch-renew-daily`: 毎週月・木 12:00 JST に Watch を自動更新（7日期限切れ防止）
  - `inbox-trigger-hourly`: 毎時 Inbox フォルダをスキャン（webhook 未検知のファイルを補完処理）
- **並行処理対応**: 3層の重複処理防止機構
  - インメモリロック（`sync.Mutex`）による同一ファイルIDの排他制御
  - Drive Properties による処理済みマーカー（早期設定）
  - 単一インスタンス構成（`max-instances=1`）でインメモリロック全体適用
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
|   |   |   +-- discord_notifier.go # Discord Webhook 通知
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
| 認証 | **SA + OAuth 二重認証** (SA: ファイル操作, OAuth: Docs/Calendar/Tasks/Photos) |
| シークレット管理 | Google Secret Manager |
| メッセージング | LINE Bot SDK v7 |
| 通知 | Discord Webhook |
| コンテナ | Docker (マルチステージ Alpine) |
| PDF 処理 | poppler-utils (pdftoppm) |
| ログ | log/slog (Cloud Logging 互換 JSON) |
| 自動運用 | Cloud Scheduler (Watch 更新 + Inbox スキャン) |

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
| `OAUTH_REFRESH_TOKEN` | **OAuth リフレッシュトークン** (Drive / Photos / Calendar / Tasks / Docs) - NotebookLM 作成に必須 |
| `ADMIN_TOKEN` | 管理エンドポイント認証トークン |
| `DRIVE_WEBHOOK_TOKEN` | Drive Watch webhook 検証トークン |
| `LINE_CHANNEL_SECRET` | LINE Bot チャンネルシークレット（オプション） |
| `LINE_CHANNEL_ACCESS_TOKEN` | LINE Bot チャンネルアクセストークン（オプション） |
| `DISCORD_WEBHOOK_URL` | Discord Webhook URL（オプション） |

### オプション

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `ADMIN_AUTH_MODE` | `required` | 管理認証モード (`required` / `optional` / `disabled`) |
| `ENABLE_COMBINED_GEMINI` | `true` | 統合 Gemini 呼び出しの有効化（分類・予定・OCR を 1 回の API 呼び出しで実行） |
| `LOG_FORMAT` | `json` | ログ形式 (`json` で Cloud Logging 互換 JSON, `text` で人間可読） |
| `LOG_LEVEL` | `info` | ログレベル (`debug` / `info` / `warn` / `error`) |
| `WEBHOOK_URL` | 自動生成 | Drive Watch webhook URL の明示指定 |
| `PORT` | `8080` | サーバーポート |

## Cloud Run デプロイ設定

| 項目 | 値 |
|------|-----|
| メモリ | 384Mi |
| CPU | 1 |
| 同時実行数 | 80 |
| 最大インスタンス | 1（重複処理防止のため単一インスタンス構成） |
| 最小インスタンス | 0（スケール to ゼロ） |
| タイムアウト | 540s |
| リージョン | asia-northeast1 |

## Cloud Scheduler 自動運用

| ジョブ名 | スケジュール | エンドポイント | 目的 |
| --------- | ------------- | -------------- | ------ |
| `watch-renew-daily` | 毎週月・木 12:00 JST | `/admin/watch/renew` | Drive Watch を定期更新（7日期限切れ防止） |
| `inbox-trigger-hourly` | 毎時 0分 UTC | `/trigger/inbox` | Inbox フォルダの定期スキャン（webhook 漏れ対策） |

**認証**: 両ジョブとも `ADMIN_TOKEN` をヘッダー認証で使用（OIDC ではない）

**冗長性**:

- インスタンス起動時に自動で Watch 開始 → 7日以内の再起動で自動復旧
- Scheduler による定期更新 → 長期稼働時も Watch が切れない
- 毎時 Inbox スキャン → webhook 未検知のファイルも確実に処理

## セットアップ

### 前提条件

- GCP プロジェクト作成済み
- OAuth 2.0 クライアント（デスクトップアプリ）作成済み
- Drive / Docs / Calendar / Tasks / Photos Library API 有効化済み
- Service Account `homedocmanager-sa@{PROJECT_ID}.iam.gserviceaccount.com` 作成済み
  - 権限: Secret Manager Secret Accessor, Cloud Run Invoker

### 1. OAuth 認証情報の取得

```bash
cd cloud-run-go
go run tools/setup_oauth.go
```

取得したリフレッシュトークンを Secret Manager に登録します。

> **重要**: NotebookLM 同期機能を使用する場合、OAuth リフレッシュトークンは**必須**です。Service Account では容量制限（15GB）により大量ドキュメントの作成ができません。

### 2. Secret Manager 登録

```bash
ADMIN_TOKEN="$(openssl rand -hex 32)"
DRIVE_WEBHOOK_TOKEN="$(openssl rand -hex 32)"

echo -n "$ADMIN_TOKEN" | gcloud secrets versions add ADMIN_TOKEN --data-file=-
echo -n "$DRIVE_WEBHOOK_TOKEN" | gcloud secrets versions add DRIVE_WEBHOOK_TOKEN --data-file=-
echo -n "YOUR_GEMINI_API_KEY" | gcloud secrets versions add GEMINI_API_KEY --data-file=-
echo -n "YOUR_REFRESH_TOKEN" | gcloud secrets versions add OAUTH_REFRESH_TOKEN --data-file=-

# LINE Bot を使用する場合（オプション）
echo -n "YOUR_LINE_CHANNEL_SECRET" | gcloud secrets versions add LINE_CHANNEL_SECRET --data-file=-
echo -n "YOUR_LINE_CHANNEL_ACCESS_TOKEN" | gcloud secrets versions add LINE_CHANNEL_ACCESS_TOKEN --data-file=-
```

**Service Account への Secret アクセス権限付与**:

```bash
gcloud secrets add-iam-policy-binding ADMIN_TOKEN \
  --member="serviceAccount:homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud secrets add-iam-policy-binding DRIVE_WEBHOOK_TOKEN \
  --member="serviceAccount:homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud secrets add-iam-policy-binding GEMINI_API_KEY \
  --member="serviceAccount:homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud secrets add-iam-policy-binding OAUTH_REFRESH_TOKEN \
  --member="serviceAccount:homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# LINE Bot 用（オプション）
gcloud secrets add-iam-policy-binding LINE_CHANNEL_SECRET \
  --member="serviceAccount:homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

gcloud secrets add-iam-policy-binding LINE_CHANNEL_ACCESS_TOKEN \
  --member="serviceAccount:homedocmanager-sa@${PROJECT_ID}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"
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
ADMIN_TOKEN="$(gcloud secrets versions access latest --secret=ADMIN_TOKEN)"
curl -i "$SERVICE_URL/admin/ping" -H "Authorization: Bearer $ADMIN_TOKEN"

# Watch 状態確認（自動起動されているはず）
curl -sS "$SERVICE_URL/admin/watch/status" -H "Authorization: Bearer $ADMIN_TOKEN"
```

### 5. Cloud Scheduler セットアップ

Drive Watch の自動更新と Inbox スキャンのため、Cloud Scheduler を設定します。

```bash
PROJECT_ID="your-project-id"
SERVICE_URL="https://homedocmanager-go-{PROJECT_NUMBER}.asia-northeast1.run.app"
ADMIN_TOKEN="$(gcloud secrets versions access latest --secret=ADMIN_TOKEN)"

# Watch 自動更新（毎週月・木 12:00 JST）
gcloud scheduler jobs create http watch-renew-daily \
  --schedule="0 12 * * 1,4" \
  --time-zone="Asia/Tokyo" \
  --uri="${SERVICE_URL}/admin/watch/renew" \
  --http-method=POST \
  --headers="Content-Type=application/json,Authorization=Bearer ${ADMIN_TOKEN}" \
  --location=asia-northeast1 \
  --project="${PROJECT_ID}"

# Inbox 定期スキャン（毎時）
gcloud scheduler jobs create http inbox-trigger-hourly \
  --schedule="0 * * * *" \
  --time-zone="UTC" \
  --uri="${SERVICE_URL}/trigger/inbox" \
  --http-method=GET \
  --headers="Authorization=Bearer ${ADMIN_TOKEN}" \
  --location=asia-northeast1 \
  --project="${PROJECT_ID}"
```

**ジョブの手動実行テスト**:

```bash
gcloud scheduler jobs run watch-renew-daily --location=asia-northeast1
gcloud scheduler jobs run inbox-trigger-hourly --location=asia-northeast1
```

## トラブルシューティング

### NotebookLM 同期が失敗する（storageQuotaExceeded）

**原因**: OAuth リフレッシュトークンが未設定または失効し、Service Account にフォールバックしている（SA は 15GB 容量制限）

**確認方法**: `GET /admin/info` のレスポンスで認証ユーザーを確認
- OAuth 正常時: `"emailAddress": "your-email@gmail.com"` （個人アカウント）
- SA フォールバック時: `"emailAddress": "homedocmanager-sa@...iam.gserviceaccount.com"`

**解決策**:

1. `tools/setup_oauth.go` でリフレッシュトークンを再取得
2. Secret Manager に `OAUTH_REFRESH_TOKEN` を登録
3. Service Account に Secret アクセス権限を付与
4. **重要**: `deploy-cloudbuild.sh` の `OAUTH_CLIENT_ID` / `OAUTH_CLIENT_SECRET` がデフォルト値として設定されていることを確認
5. 再デプロイ

### OAuth リフレッシュトークンが失効する（invalid_grant）

**症状**: ログに `Token has been expired or revoked` が出力され、Drive / Calendar / Tasks / Photos の全サービスが機能しない

**原因**: Google OAuth のリフレッシュトークンは以下の条件で失効する
- Google アカウントのパスワード変更
- アカウントのセキュリティ設定変更
- 6ヶ月間トークンが使用されなかった場合
- OAuth 同意画面で「テスト」モードの場合は 7 日で失効

**解決策**:

1. `tools/setup_oauth.go` でリフレッシュトークンを再取得
2. Secret Manager を更新: `echo -n "NEW_TOKEN" | gcloud secrets versions add OAUTH_REFRESH_TOKEN --data-file=-`
3. 再デプロイ（または新リビジョンで Secret の再読み込み）
4. `GET /admin/info` で OAuth ユーザーが表示されることを確認

### Drive Watch が動作しない

**症状**: Inbox にファイルを追加しても処理されない

**確認手順**:

1. Watch 状態確認: `GET /admin/watch/status`
2. ログ確認: `gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go"`
3. Cloud Scheduler ジョブの実行履歴確認

**解決策**:

- インスタンスは起動時に自動で Watch を開始するため、サービス再起動で復旧する場合が多い
- Cloud Scheduler `watch-renew-daily` が正常に実行されているか確認
- `DRIVE_WEBHOOK_TOKEN` が正しく設定されているか確認

### LINE Bot が反応しない

**原因**: LINE シークレットが Secret Manager に登録されていない、または SA 権限がない

**解決策**:

1. `LINE_CHANNEL_SECRET` と `LINE_CHANNEL_ACCESS_TOKEN` を Secret Manager に登録
2. Service Account に Secret アクセス権限を付与
3. 再デプロイ
4. ログで `LINE Bot Webhook registered at /callback` が出力されることを確認

### Google Tasks にタスクが重複して作成される

**原因**: 複数のCloud Runインスタンスが並行起動し、同じファイルを重複処理している

**解決策**:

- リビジョン 00052 以降では、以下の3層の重複防止機構により完全に解消:
  1. **インメモリロック**: 同一ファイルIDの同時処理を `sync.Mutex` で排他制御
  2. **早期マーキング**: ファイル処理の最初に Drive Properties で処理済みマークを設定
  3. **単一インスタンス構成**: `max-instances=1` によりインメモリロックが全リクエストで有効
- ログで「別のリクエストで処理中のためスキップ」または「既に処理済みのファイルです」が表示されることを確認
- 既に作成された重複タスクは手動で削除が必要

## テスト

```bash
cd cloud-run-go
go test ./...
```

## アーキテクチャ上の注意点

### OAuth + Service Account 二重認証アーキテクチャ

本アプリケーションは **SA（ファイル操作）+ OAuth（API制限操作）** の二重認証モデルを採用しています:

#### Service Account で実行:
- Drive ファイルの読み取り・移動・リネーム・プロパティ更新
- Drive Changes API（Watch通知）による変更検知
- フォルダ作成・検索

#### OAuth で実行（ユーザーアカウント必須）:
- NotebookLM 用 Google Docs 作成・編集（`drive.file`, `documents` スコープ）
- Google Photos アップロード（`photoslibrary.appendonly` スコープ）
- Google Calendar イベント作成（`calendar.events` スコープ）
- Google Tasks 作成（`tasks` スコープ）

#### なぜ二重認証が必要か:
- **SA の容量制限**: Service Account の Drive 容量は 15GB に制限されており、NotebookLM 統合ドキュメントを大量作成できない
- **OAuth の Restricted スコープ回避**: `drive` スコープ（全ファイルアクセス）は Google の CASA 監査（$15,000+）が必要なため、`drive.file` スコープ（アプリ作成ファイルのみ）+ SA で代替
- **SA フォールバック**: OAuth が設定されていない場合は SA にフォールバックするが、NotebookLM 同期は容量制限により失敗する可能性あり

#### 認証状態の確認:
```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" https://your-service-url/admin/info
```
- OAuth 正常時: `"emailAddress": "your-email@gmail.com"`
- SA フォールバック時: `"emailAddress": "homedocmanager-sa@...iam.gserviceaccount.com"`

### Cloud Run のスケール to ゼロ

- `min-instances=0` のため、リクエストがない期間はインスタンスが停止
- 起動時に自動で Drive Watch を開始するため、コールドスタート後も自動復旧
- Cloud Scheduler により定期的に Watch 更新・Inbox スキャンが実行されるため、長期稼働時も安定

### Gemini API の統合呼び出し

- `ENABLE_COMBINED_GEMINI=true` で、分類・予定抽出・OCR を 1 回の API 呼び出しで実行
- 入力トークン削減により Gemini API コストを約 66% 削減
- 個別呼び出しとの互換性を保つため、フォールバック処理も実装

## ライセンス

Private Use Only
