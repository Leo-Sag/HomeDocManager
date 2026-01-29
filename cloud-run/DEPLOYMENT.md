# Cloud Runへのデプロイ前チェックリスト

## 必須設定

- [ ] GCPプロジェクトIDを設定
- [ ] 必要なAPIを有効化
  - [ ] Google Drive API
  - [ ] Google Photos API
  - [ ] Cloud Run API
  - [ ] Secret Manager API
  - [ ] Pub/Sub API
  - [ ] Cloud Build API
- [ ] OAuth 2.0クライアントIDを作成
- [ ] `scripts/setup_oauth.py`を実行してリフレッシュトークンを取得
- [ ] Secret Managerに以下を登録
  - [ ] GEMINI_API_KEY
  - [ ] PHOTOS_REFRESH_TOKEN
- [ ] `.env`ファイルを作成（`.env.example`を参考に）
- [ ] `config/settings.py`のフォルダIDを確認・更新

## デプロイ手順

1. 環境変数を設定
```bash
export GCP_PROJECT_ID=your-project-id
export GCP_REGION=asia-northeast1
```

2. デプロイスクリプトを実行
```bash
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

3. Pub/Subトピックとサブスクリプションを作成
```bash
gcloud pubsub topics create drive-events
gcloud pubsub subscriptions create drive-events-sub \
  --topic drive-events \
  --push-endpoint=https://[YOUR-CLOUD-RUN-URL]/
```

4. Google Drive通知を設定
```bash
# Google Drive APIのwatch機能を使用
# 詳細は実装計画を参照
```

## テスト

1. ヘルスチェック
```bash
curl https://[YOUR-CLOUD-RUN-URL]/health
```

2. 手動テスト
```bash
curl -X POST https://[YOUR-CLOUD-RUN-URL]/test \
  -H "Content-Type: application/json" \
  -d '{"file_id": "your-test-file-id"}'
```

## 本番運用前の確認

- [ ] ログが正しく出力されているか
- [ ] コスト見積もりを確認
- [ ] エラーハンドリングが適切に動作するか
- [ ] GASトリガーを停止する準備
