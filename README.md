# HomeDocManager (Smart Document Filing System)

Gemini AIを活用して、Googleドライブ上のドキュメント（PDF/画像）を自動で解析・リネーム・分別するシステムです。
また、領収書やチラシなどの画像は Googleフォト へ自動で同期され、行事系ドキュメントは Googleカレンダー/Tasks へ自動登録されます。
NotebookLMとの連携機能も搭載し、OCR結果を自動的にドキュメント化して蓄積します。

## システム構成

本システムは **Google Cloud Run (Python)** を核として動作します。不要になったGASコードはアーカイブされています。

### 1. Core Logic (Cloud Run)
- **場所:** `/cloud-run`
- **技術:** Python 3.11, Flask, Gunicorn
- **機能:**
  - **Gemini 1.5 Pro/Flash:** 画像/PDFの内容を解析し、ファイル名とカテゴリを決定
  - **Google Drive API:** ファイルの移動・リネーム・NotebookLM用ドキュメント作成
  - **Google Photos API:** 画像のアップロード
  - **Google Calendar/Tasks API:** 行事やタスクの自動抽出と登録
    - 子供の名前（クラス名）検出機能付き
    - エラー時のTasks通知機能
  - **NotebookLM Sync:** 年別・カテゴリ別の累積ドキュメント自動生成

### 2. Triggers
- **Trigger Endpoint:** `/trigger/inbox` (Cloud Run)
  - Inbox内のファイルを一括処理するためのエンドポイント
- **Legacy Trigger (Reference):** `/cloud-run/scripts/Trigger.gs`
  - 過去のGASトリガーコード（参考用）

## ディレクトリ構造

```
HomeDocManager/
├── cloud-run/              # Pythonアプリケーション本体 (Core)
│   ├── modules/            # 各種クライアント・ロジック
│   ├── config/             # 設定ファイル (settings.py)
│   ├── scripts/            # デプロイ用スクリプト
│   ├── Dockerfile          # コンテナ定義
│   └── requirements.txt    # Python依存ライブラリ
├── _archive/               # 過去の遺産コード (GAS版など)
└── README.md               # 本ファイル
```

## 更新履歴
- **v2.0.0 (2026-01-30)**:
  - **Full Cloud Run Migration**: 全機能をPython化しGASを廃止（アーカイブ化）
  - **Adult Document Support**: 大人（祖父母・両親）の書類仕分けに対応
  - **NotebookLM Sync V2**: 累積ドキュメント方式に変更、ソースリンク自動挿入
  - **Admin Tools**: 容量クリーンアップ、Inboxトリガーエンドポイント実装
- **v1.2.0 (2026-01-29)**: CalendarSync機能をGASからCloud Runへ移行
- **v1.0.0**: Cloud Run移行完了

## ライセンス
Private Use Only
