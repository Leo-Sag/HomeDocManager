# Cloud Run ドキュメント管理システム

ScanSnap iX1300とGemini 3を使用したPCレス・ドキュメント管理システムのCloud Run実装

## 概要

このプロジェクトは、既存のGoogle Apps Script（GAS）ベースのドキュメント管理システムをCloud Run基盤に移行したものです。

### 主な機能

- **AIルーターパターン**: Gemini 3 Flash優先、信頼度スコアに基づくProへのエスカレーション
- **PDF処理**: poppler-utilsを使用した高度なPDF→画像変換
- **Google Photos連携**: OAuth 2.0による2段階アップロードプロトコル
- **イベント駆動**: Pub/Subトリガーによるリアルタイム処理
- **コスト最適化**: 月間1,000枚処理で約150円（無料枠内）

## プロジェクト構造

```
cloud-run/
├── main.py                    # Cloud Runエントリーポイント
├── requirements.txt           # Python依存関係
├── Dockerfile                 # コンテナ定義
├── .env.example              # 環境変数テンプレート
├── config/
│   └── settings.py           # 設定ファイル（Config.gsから移植）
├── modules/
│   ├── ai_router.py          # Gemini Flash/Proルーター
│   ├── pdf_processor.py      # PDF→画像変換
│   ├── drive_client.py       # Google Drive API
│   ├── photos_client.py      # Google Photos API
│   └── file_sorter.py        # FileSorter機能
├── utils/
│   └── logger.py             # ロギング設定
└── scripts/
    ├── setup_oauth.py        # OAuth認証セットアップ
    └── deploy.sh             # デプロイスクリプト
```

## セットアップ

### 1. 環境変数の設定

`.env.example`をコピーして`.env`を作成し、必要な値を設定:

```bash
cp .env.example .env
```

### 2. GCP APIの有効化

```bash
gcloud services enable \
  drive.googleapis.com \
  photoslibrary.googleapis.com \
  run.googleapis.com \
  secretmanager.googleapis.com \
  pubsub.googleapis.com \
  cloudbuild.googleapis.com
```

### 3. OAuth 2.0認証の設定

Google Photos API用のリフレッシュトークンを取得:

```bash
cd scripts
python setup_oauth.py
```

### 4. Secret Managerへの登録

Gemini APIキーをSecret Managerに登録:

```bash
echo -n "your-gemini-api-key" | gcloud secrets create GEMINI_API_KEY --data-file=-
```

### 5. デプロイ

```bash
chmod +x scripts/deploy.sh
./scripts/deploy.sh
```

## ローカル開発

### 依存関係のインストール

```bash
pip install -r requirements.txt
```

### ローカルサーバーの起動

```bash
export GCP_PROJECT_ID=your-project-id
export GEMINI_API_KEY=your-api-key
export PHOTOS_REFRESH_TOKEN=your-refresh-token
export OAUTH_CLIENT_ID=your-client-id
export OAUTH_CLIENT_SECRET=your-client-secret

python main.py
```

### テストエンドポイント

```bash
curl -X POST http://localhost:8080/test \
  -H "Content-Type: application/json" \
  -d '{"file_id": "your-google-drive-file-id"}'
```

## Pub/Sub設定

### トピックの作成

```bash
gcloud pubsub topics create drive-events
```

### サブスクリプションの作成

```bash
gcloud pubsub subscriptions create drive-events-sub \
  --topic drive-events \
  --push-endpoint=https://your-cloud-run-url/
```

## モニタリング

### ログの確認

```bash
gcloud run logs read document-processor --region asia-northeast1
```

### メトリクスの確認

GCPコンソール > Cloud Run > document-processor > メトリクス

## トラブルシューティング

### Gemini API呼び出しエラー

- Secret ManagerにGEMINI_API_KEYが正しく設定されているか確認
- APIキーの権限を確認

### Google Photos アップロードエラー

- リフレッシュトークンが有効か確認
- OAUTH_CLIENT_IDとOAUTH_CLIENT_SECRETが正しく設定されているか確認

### PDF変換エラー

- Dockerfileにpoppler-utilsが含まれているか確認
- メモリ設定を確認（1Gi以上推奨）

## コスト見積もり

月間1,000枚のドキュメント処理を想定:

| 項目 | 月額コスト |
|------|-----------|
| Cloud Run | ~$0.24（無料枠内なら$0） |
| Cloud Storage | ~$0.10 |
| Gemini 3 Flash | ~$0.02 |
| Gemini 3 Pro（10%） | ~$0.01 |
| Pub/Sub, Secret Manager | 無視できるレベル |
| **合計** | **~$1.00以下（約150円）** |

## ライセンス

MIT License
