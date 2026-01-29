# HomeDocManager (Smart Document Filing System)

Gemini AIを活用して、Googleドライブ上のドキュメント（PDF/画像）を自動で解析・リネーム・分別するシステムです。
また、領収書やチラシなどの画像は Googleフォト へ自動で同期されます。

## システム構成

本システムは **Google Cloud Run (Python)** と **Google Apps Script (GAS)** のハイブリッド構成で動作します。

### 1. Core Logic (Cloud Run)
- **場所:** `/cloud-run`
- **技術:** Python 3.11, Flask, Gunicorn
- **機能:**
  - **Gemini 1.5/3.0 API:** 画像/PDFの内容を解析し、ファイル名とカテゴリを決定
  - **Google Drive API:** ファイルの移動・リネーム
  - **Google Photos API:** 画像のアップロード
  - **Secret Manager:** APIキーなどの機密情報管理
  - **Pub/Sub:** 非同期処理のためのメッセージング

### 2. Triggers (Google Apps Script)
- **場所:** `/cloud-run/scripts/Trigger.gs` (およびルート直下の `.gs` ファイル)
- **機能:**
  - Googleドライブの特定フォルダ（`00_受領箱`）を監視
  - 新規ファイルがアップロードされると、Cloud Runのエンドポイントへ通知（Pub/Sub経由または直接HTTP呼び出し）
  - `CalendarSync.gs`: 書類から日付情報を抽出してカレンダー登録（※旧機能または併用）

## セットアップ手順

### 必要要件
- Google Cloud Platform (GCP) プロジェクト
- Cloud Run / Cloud Build / Secret Manager / Pub/Sub の有効化
- Gemini API キー

### デプロイ (Cloud Run)
`/cloud-run` ディレクトリ内で、以下のスクリプトを使用してデプロイします。
```bash
# Windows (PowerShell)
./scripts/deploy.ps1
```
※ APIキーなどの環境変数は GCP Secret Manager に設定されています。

## ディレクトリ構造

```
HomeDocManager/
├── cloud-run/              # Pythonアプリケーション本体
│   ├── modules/            # AIRouter, DriveClient, PhotosClientなどのモジュール
│   ├── config/             # 設定ファイル (settings.py)
│   ├── scripts/            # デプロイ用スクリプト & GASトリガーコード
│   ├── Dockerfile          # コンテナ定義
│   └── requirements.txt    # Python依存ライブラリ
├── CalendarSync.gs         # (GAS) カレンダー連携用スクリプト
├── Config.gs               # (GAS) 共有設定
└── NotebookLMSync.gs       # (GAS) NotebookLM連携用スクリプト
```

## 更新履歴
- **v1.0.0**: Cloud Run移行完了。Gemini API連携、Google Photosアップロード機能、堅牢なリトライロジックを実装。
- **Revision 00017-00018**: 安定稼働バージョン（メモリ最適化済み）。

## ライセンス
Private Use Only
