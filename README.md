# HomeDocManager (Smart Document Filing System)

Gemini AIを活用して、Googleドライブ上のドキュメント（PDF/画像）を自動で解析・リネーム・分別するシステムです。
また、領収書やチラシなどの画像は Googleフォト へ自動で同期され、行事系ドキュメントは Googleカレンダー/Tasks へ自動登録されます。

## システム構成

本システムは **Google Cloud Run (Python)** と **Google Apps Script (GAS)** のハイブリッド構成で動作します。

### 1. Core Logic (Cloud Run)
- **場所:** `/cloud-run`
- **技術:** Python 3.11, Flask, Gunicorn
- **機能:**
  - **Gemini 1.5/3.0 API:** 画像/PDFの内容を解析し、ファイル名とカテゴリを決定
  - **Google Drive API:** ファイルの移動・リネーム
  - **Google Photos API:** 画像のアップロード
  - **Google Calendar/Tasks API:** 行事やタスクの自動抽出と登録
    - 子供の名前が検出された場合、イベント名に `【名前】` を付与
    - 指定されたカレンダーIDへの登録
  - **Secret Manager:** APIキーなどの機密情報管理
  - **Pub/Sub:** 非同期処理のためのメッセージング

### 2. Triggers / Secondary Logic (GAS)
- **Trigger.gs:** (`/cloud-run/scripts/Trigger.gs`)
  - Googleドライブの `00_Inbox` フォルダを監視
  - 新規ファイルがアップロードされると Cloud Run へ Pub/Sub 通知を行う
- **NotebookLMSync.gs:** (`/NotebookLMSync.gs`)
  - 処理済みファイルを NotebookLM 用に OCR 変換（ドキュメント化）し、専用フォルダへ同期
  - **設定ファイル:** `Config.gs`

## ディレクトリ構造

```
HomeDocManager/
├── cloud-run/              # Pythonアプリケーション本体 (Core)
│   ├── modules/            # 各種クライアント・ロジック (AIRouter, Calendar, Tasksなど)
│   ├── config/             # Python設定ファイル (settings.py)
│   ├── scripts/            # デプロイ用スクリプト & GASトリガーコード
│   ├── Dockerfile          # コンテナ定義
│   └── requirements.txt    # Python依存ライブラリ
├── NotebookLMSync.gs       # (GAS) NotebookLM連携用スクリプト
├── Config.gs               # (GAS) NotebookLMSync用設定
├── _archive/               # 過去の遺産コード
└── README.md               # 本ファイル
```

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

## 更新履歴
- **v1.2.0 (2026-01-29)**: 
  - CalendarSync機能をGASからCloud Runへ完全移行
  - イベント・タスク登録時に子供の名前を自動付与する機能追加
  - カレンダーIDの指定機能追加
- **v1.0.0**: Cloud Run移行完了。Gemini API連携、Google Photosアップロード機能。

## ライセンス
Private Use Only
