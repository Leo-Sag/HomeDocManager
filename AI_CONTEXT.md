# Project Context & Maintenance Guide

このファイルは、将来のAIアシスタントや開発者が本プロジェクト（HomeDocManager）を理解し、保守・拡張するための重要な情報をまとめたものです。

## 1. システム概要
HomeDocManagerは、Googleドライブ上のドキュメントを整理し、必要な情報を抽出・連携するハイブリッドシステムです。

- **アーキテクチャ:** Cloud Run (Python) + Google Apps Script (GAS)
- **主な機能:**
    - PDF/画像の自動リネーム＆フォルダ振り分け (Cloud Run)
    - Googleフォトへの画像バックアップ (Cloud Run)
    - カレンダーイベント登録 (GAS: CalendarSync)
    - NotebookLM用同期 (GAS: NotebookLMSync)

## 2. ディレクトリ構成と役割

### `cloud-run/` (Main Application)
現在稼働している中核システムです。Pythonで作られています。

- **`main.py`**: Flaskアプリケーションのエントリーポイント。HTTPリクエストを受け取ります。
- **`modules/`**:
    - `ai_router.py`: Gemini APIとの通信管理（Flash/Proモデルの切り替えロジック含む）。
    - `drive_client.py`: Google Drive API操作（ダウンロード、移動、リネーム）。**重要:** ネットワーク切断対策の強力なリトライロジックが実装されています。
    - `photos_client.py`: Google Photos API操作（画像アップロード）。
    - `file_sorter.py`: 処理全体のオーケストレーター（ダウンロード→解析→分岐→移動）。
    - `pdf_processor.py`: `pdf2image` を使ったPDFの画像変換。
- **`config/settings.py`**: 環境設定。モデル名やフォルダIDのマッピング。
- **`scripts/`**:
    - `Trigger.gs`: GAS側にデプロイし、ファイルの変更を検知してCloud Runを呼び出すトリガー。
    - `deploy.ps1`: デプロイ用スクリプト。

### `root/` (Legacy & Side Tools)
- **`CalendarSync.gs`**: 書類から日付を抽出してカレンダー登録する単体機能（Cloud Run移行対象外、現役）。
- **`NotebookLMSync.gs`**: NotebookLM連携機能（現役）。
- **`Config.gs`**: GAS側の設定ファイル。Cloud Runの `settings.py` と二重管理にならないよう注意が必要。

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
- `OAUTH_CLIENT_SECRET`: OAuth 2.0 クライアントシークレット（Photos API用）
- `PHOTOS_REFRESH_TOKEN`: Photos APIのリフレッシュトークン

### 既知のトラブルと解決策
1.  **"Broken pipe" / "SSL error"**:
    - Drive APIからのダウンロード時に頻発。`drive_client.py` 内で、リトライごとに `service` オブジェクトを再生成することで解決済み。コードを変更する際は、この再生成ロジックを削除しないこと。
2.  **APIキー認証エラー**:
    - シークレットに改行コードや余計な文字が含まれる場合がある。コード内で `.strip()` を使用してサニタイズしている。

## 4. 将来のTodo
- [ ] `CalendarSync.gs` のロジックを Cloud Run (`modules/calendar_client.py`) に移植して、GAS依存を減らす（完全なPython化）。
- [ ] Geminiモデルのアップデート（将来 `gemini-1.5` が非推奨になった場合、`settings.py` を更新）。

---
**Note to AI:**
修正を行う際は、`cloud-run` フォルダ内のファイルが最新のソースコードです。ルート直下の `.gs` ファイルはGASエディタ上のコードと同期されているか確認が必要です。
