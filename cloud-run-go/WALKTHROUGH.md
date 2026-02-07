# Cloud Run (Go) 本番反映ウォークスルー

この手順書は `HomeDocManager/cloud-run-go` の現行実装を Cloud Run に反映し、ロールバック可能な状態（Revision運用）を保つためのウォークスルーです。

対象:
- 管理系エンドポイント保護（`ADMIN_TOKEN`）
- Drive webhook 検証（`DRIVE_WEBHOOK_TOKEN`）
- 構造化ログ（`LOG_FORMAT=json`）
- NotebookLM 同期の重複抑止（同期済みマーカー）
- PDFページ順の安定化（`page-1,page-2,page-10` の数値順）
- コスト制御（メモリ/同時実行/最大インスタンス）

## 0. 前提

- ネットワークが通る環境で実行（このコンテナ環境は外部ネットワークに出られないことがあります）。
- `gcloud` がインストール済みでログイン済み。
- `docker` を使う場合: ローカルに `docker` が必要。

## 1. 必要情報の取得

最低限これを押さえます:
- `PROJECT_ID`
- `PROJECT_NUMBER`（Cloud Run のURL組み立てに使う）
- `REGION`（例: `asia-northeast1`）
- `SERVICE_NAME`（既定: `homedocmanager-go`）

取得コマンド例:

```bash
PROJECT_ID="$(gcloud config get-value project)"
PROJECT_NUMBER="$(gcloud projects describe "$PROJECT_ID" --format='value(projectNumber)')"
REGION="asia-northeast1"
SERVICE_NAME="homedocmanager-go"
```

既存サービス確認（任意）:

```bash
gcloud run services describe "$SERVICE_NAME" --region "$REGION" >/dev/null \
  && echo "service exists" || echo "service not found"
```

## 2. 本番用トークン生成（推奨）

`ADMIN_TOKEN` と `DRIVE_WEBHOOK_TOKEN` はランダムな秘密情報でOKです。
推奨は「32バイト」相当で、以下は 32バイトをhex化した 64文字トークンになります:

```bash
ADMIN_TOKEN="$(openssl rand -hex 32)"
DRIVE_WEBHOOK_TOKEN="$(openssl rand -hex 32)"
```

ログに出さず、gitにも入れません。

## 3. Secret Manager 登録（推奨）

既に存在する場合は `versions add` だけでOKです。
`gcloud secrets create --data-file=-` は「作成と同時に1つ目のバージョンも追加」されるため、
ここでは「存在確認 -> なければ作成 -> versions add」の形にしています（重複バージョンを増やさない）。

```bash
gcloud secrets describe ADMIN_TOKEN >/dev/null 2>&1 || gcloud secrets create ADMIN_TOKEN --replication-policy="automatic"
echo -n "$ADMIN_TOKEN" | gcloud secrets versions add ADMIN_TOKEN --data-file=-

gcloud secrets describe DRIVE_WEBHOOK_TOKEN >/dev/null 2>&1 || gcloud secrets create DRIVE_WEBHOOK_TOKEN --replication-policy="automatic"
echo -n "$DRIVE_WEBHOOK_TOKEN" | gcloud secrets versions add DRIVE_WEBHOOK_TOKEN --data-file=-
```

Gemini / OAuth refresh token を Secret Manager で運用する場合:

```bash
gcloud secrets describe GEMINI_API_KEY >/dev/null 2>&1 || gcloud secrets create GEMINI_API_KEY --replication-policy="automatic"
echo -n "YOUR_GEMINI_API_KEY" | gcloud secrets versions add GEMINI_API_KEY --data-file=-

gcloud secrets describe OAUTH_REFRESH_TOKEN >/dev/null 2>&1 || gcloud secrets create OAUTH_REFRESH_TOKEN --replication-policy="automatic"
echo -n "YOUR_OAUTH_REFRESH_TOKEN" | gcloud secrets versions add OAUTH_REFRESH_TOKEN --data-file=-
```

注:
- `OAUTH_CLIENT_ID` / `OAUTH_CLIENT_SECRET` は現状アプリ側が環境変数から参照します（Secret Manager運用するならデプロイ方法に合わせて `--set-secrets` で注入してください）。

## 4. 非シークレット設定（環境変数）

推奨:
- `ADMIN_AUTH_MODE=required`
- `LOG_FORMAT=json`（JSONログにしたい場合）
- `LOG_LEVEL=info`

任意:
- `ENABLE_COMBINED_GEMINI=true|false`

## 5. デプロイ（`cloud-run-go/` から実行）

`cloud-run-go` には2つのスクリプトがあります。

- `deploy.sh`: ローカルDockerでビルドして push する
- `deploy-cloudbuild.sh`: Cloud Buildでビルドして push する（Docker不要）

### Option A: `deploy.sh`（Dockerあり）

1. `HomeDocManager/cloud-run-go/deploy.sh` の `PROJECT_ID` / `PROJECT_NUMBER` が正しいか確認（必要なら書き換え）
2. 必要な環境変数をセットして実行

Secret Manager を使う場合（推奨）:

```bash
cd HomeDocManager/cloud-run-go

export USE_SECRET_MANAGER=1
export ADMIN_AUTH_MODE=required
export LOG_FORMAT=json
export LOG_LEVEL=info

./deploy.sh
```

環境変数で直接トークンを渡す場合（Secret Managerを使わない）:

```bash
cd HomeDocManager/cloud-run-go

export ADMIN_AUTH_MODE=required
export ADMIN_TOKEN="$ADMIN_TOKEN"
export DRIVE_WEBHOOK_TOKEN="$DRIVE_WEBHOOK_TOKEN"
export LOG_FORMAT=json
export LOG_LEVEL=info

./deploy.sh
```

### Option B: `deploy-cloudbuild.sh`（Dockerなし）

`deploy-cloudbuild.sh` は `--set-secrets` を使うため、事前に Secret Manager に以下が存在する必要があります:
- `ADMIN_TOKEN`
- `DRIVE_WEBHOOK_TOKEN`
- `GEMINI_API_KEY`

`OAUTH_REFRESH_TOKEN` は存在すれば注入されます（未作成の場合でもデプロイは継続します）。

```bash
cd HomeDocManager/cloud-run-go

export ADMIN_AUTH_MODE=required
export LOG_FORMAT=json
export LOG_LEVEL=info

./deploy-cloudbuild.sh
```

補足:
- スクリプトは `--allow-unauthenticated` を付けています（Pub/Sub push等の都合）。
- 管理系は `ADMIN_TOKEN` で保護されます。

## 6. デプロイ後の確認

サービスURL取得:

```bash
SERVICE_URL="$(gcloud run services describe "$SERVICE_NAME" --region "$REGION" --format='value(status.url)')"
echo "$SERVICE_URL"
```

ヘルスチェック:

```bash
curl -sS "$SERVICE_URL/health"
```

管理系認証の確認:

```bash
curl -i "$SERVICE_URL/admin/ping"

curl -i "$SERVICE_URL/admin/ping" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

Drive webhook トークン検証（誤トークンなら `403` が期待）:

```bash
curl -i -X POST "$SERVICE_URL/webhook/drive" \
  -H "X-Goog-Channel-Token: wrong" \
  -H "X-Goog-Channel-ID: dummy" \
  -H "X-Goog-Resource-ID: dummy"
```

## 7. ロールバック（Cloud Run Revision）

Revision一覧:

```bash
gcloud run revisions list --service "$SERVICE_NAME" --region "$REGION"
```

1つ前など、特定Revisionへ 100% 戻す:

```bash
gcloud run services update-traffic "$SERVICE_NAME" --region "$REGION" --to-revisions REVISION_NAME=100
```

設定だけ戻す（コードは戻さない）例:
- 管理系認証を無効化: `ADMIN_AUTH_MODE=disabled`
- 統合Geminiを無効化: `ENABLE_COMBINED_GEMINI=false`
- JSONログをやめる: `LOG_FORMAT=text`

## 8. ローカル検証（任意）

```bash
cd HomeDocManager/cloud-run-go
go test ./...
```
