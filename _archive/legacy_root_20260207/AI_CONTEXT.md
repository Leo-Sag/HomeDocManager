# Project Context & Maintenance Guide

このファイルは、将来のAIアシスタントや開発者が本プロジェクト（HomeDocManager）を理解し、保守・拡張するための重要な情報をまとめたものです。

## 1. システム概要
HomeDocManagerは、Googleドライブ上のドキュメントを整理し、必要な情報を抽出・連携するハイブリッドシステムです。

- **アーキテクチャ:** Cloud Run (Python) + Google Apps Script (GAS)
- **主な機能:**
    - PDF/画像の自動リネーム＆フォルダ振り分け (Cloud Run)
    - Googleフォトへの画像バックアップ (Cloud Run)
    - **カレンダー・タスク自動登録 (Cloud Run)**: v1.2.0よりPython化完了
    - NotebookLM用同期 (GAS: NotebookLMSync)

## 2. ディレクトリ構成と役割

### `cloud-run/` (Main Application)
現在稼働している中核システムです。Pythonで作られています。

- **`main.py`**: Flaskアプリケーションのエントリーポイント。HTTPリクエストを受け取ります。
- **`modules/`**:
    - `ai_router.py`: Gemini APIとの通信管理（Flash/Proモデルの切り替えロジック含む）。
    - `drive_client.py`: Google Drive API操作（ダウンロード、移動、リネーム）。**重要:** ネットワーク切断対策の強力なリトライロジックが実装されています。
    - `photos_client.py`: Google Photos API操作（画像アップロード）。
    - `calendar_client.py`: Google Calendar API操作（イベント登録）。
    - `tasks_client.py`: Google Tasks API操作（タスク登録）。
    - `file_sorter.py`: 処理全体のオーケストレーター（ダウンロード→解析→分岐→移動→カレンダー/タスク登録）。
    - `pdf_processor.py`: `pdf2image` を使ったPDFの画像変換。
- **`config/settings.py`**: 環境設定。モデル名やフォルダIDのマッピング。
- **`scripts/`**:
    - `Trigger.gs`: GAS側にデプロイし、ファイルの変更を検知してCloud Runを呼び出すトリガー。
    - `deploy.ps1`: デプロイ用スクリプト。

### `root/` (Legacy & Side Tools)
- **`NotebookLMSync.gs`**: NotebookLM連携機能（現役）。
- **`Config.gs`**: GAS側の設定ファイル (`NotebookLMSync.gs` 専用)。
- **`_archive/`**: 旧コード (`CalendarSync.gs` など)

## 3. 運用・保守運用

### デプロイ方法
変更を加えた場合は、必ずバージョン（リビジョン）履歴を残しつつデプロイしてください。
```powershell
cd cloud-run
./scripts/deploy.ps1
```

### 重要な設定 (Secret Manager)
Cloud Runは以下のシークレットを使用します。
- `GEMINI_API_KEY`: Gemini APIキー
- `OAUTH_CLIENT_SECRET`: OAuth 2.0 クライアントシークレット
- **`OAUTH_REFRESH_TOKEN`**: Photos/Calendar/Tasks兼用のリフレッシュトークン（v1.2.0〜）
- `PHOTOS_REFRESH_TOKEN`: (Legacy) 旧Photos専用トークン。現在は互換性のためにコードに残っているが、推奨されない。

### 既知のトラブルと解決策
1.  **"Broken pipe" / "SSL error"**:
    - Drive APIからのダウンロード時に頻発。`drive_client.py` 内で、リトライごとに `service` オブジェクトを再生成することで解決済み。コードを変更する際は、この再生成ロジックを削除しないこと。
2.  **OAuth認証エラー**:
    - 認証スコープが不足している場合がある。`setup_oauth.py` を実行して新しいスコープ（Calendar/Tasks等）を含むトークンを再発行する必要がある。

## 4. 将来のTodo
- [ ] NotebookLMSyncロジックのCloud Run移行検討（完全Python化するか、GASの利便性を取るか判断）。
- [ ] Geminiモデルのアップデート（将来 `gemini-1.5` が非推奨になった場合、`settings.py` を更新）。
- [ ] テストコードの拡充（現在は手動アップロード確認が主）。

---
**Note to AI:**
修正を行う際は、`cloud-run` フォルダ内のファイルが最新のソースコードです。ルート直下の `.gs` ファイルは `Trigger.gs` を除き、スタンドアロンのツールとして動作しています。
