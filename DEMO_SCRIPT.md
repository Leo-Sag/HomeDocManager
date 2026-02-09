# HomeDocManager - デモ動画台本

**対象**: Google Sensitive Scope 検証用デモ動画

---

## 1. イントロダクション（30秒）

**台本:**
"This is HomeDocManager - an intelligent document management system for families. It automatically sorts, categorizes, and organizes household documents from Google Drive using AI, then syncs important information to Google Calendar, Tasks, Google Docs, Google Photos, and other services.

Let me show you how each permission is used in the application."

**画面:**
- デスクトップ全体を表示
- YouTube にアップロードする動画であることを明示

---

## 2. OAuth 同意画面のデモ（1分）

### 2.1 認証フローの開始

**台本:**
"First, let me show you the OAuth authentication flow. When a user starts the application for the first time, they see a browser window with Google's authentication page."

**アクション:**
1. ブラウザを開く
2. OAuth 認証 URL にアクセス（またはスクリーンショット表示）
3. 「Sign in with Google」をクリック

### 2.2 同意画面の表示

**台本:**
"Google then presents a consent screen showing exactly which permissions the app requests."

**アクション:**
1. Google ログイン画面でアカウント選択
2. **同意画面を明確に見せる** - 画面に表示されるスコープ一覧：
   - `photos.appendonly` - Upload photos to Google Photos
   - `calendar.events` - Manage your Google Calendar events
   - `tasks` - Manage your Google Tasks
   - `documents` - Create and edit your Google Docs
   - `drive.file` - View and manage files created by this app

**台本:**
"The user reviews these permissions and understands exactly what access is being granted. Then they click 'Allow' to authorize the application."

3. 「許可」をクリック
4. リダイレクト成功ページを表示

---

## 3. ファイル処理フロー（2分 30秒）

### 3.1 Inbox にファイルをアップロード

**台本:**
"Now let me demonstrate how the application uses these permissions. First, I'll place a sample document in the Inbox folder. This could be a school notice, medical bill, or any household document."

**アクション:**
1. Google Drive を開く
2. `00_Inbox` フォルダを開く
3. サンプルファイル（PDF または画像）をアップロード
4. ファイルが正常にアップロードされたことを確認

### 3.2 ファイル処理の実行確認

**台本:**
"The application monitors this folder in real-time using Drive Watch API. When a new file appears, the system automatically analyzes its content and processes it according to its category."

**アクション:**
1. Cloud Run のログを確認（または管理画面）
   - `GET /admin/info` でエンドポイント確認
   - ログで処理開始を確認

```
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go" --limit=20
```

2. ログで以下の処理順序を見せる：
   - ファイル検知
   - OCR 実行
   - Gemini AI による分類
   - カテゴリ判定完了

---

## 4. 各スコープの実装例（セグメント別）

### 4.1 Google Drive - `drive.file` スコープ

**台本:**
"The app uses the 'drive.file' permission to create a unified document that syncs information extracted from processed documents. This document is stored in the user's Drive."

**アクション:**
1. Google Drive の NotebookLM 同期フォルダを開く
2. 新しく作成された Google Doc（例: `2026年度_医療`）を表示
3. ファイルが正常に作成されたことを確認
4. **重要**: 「このファイルはこのアプリが作成した」ことを明示

**スコープの正当性:**
- ✅ ファイル作成: NotebookLM 同期用 Google Doc の自動作成
- ✅ ファイル情報取得: 既存ファイルの確認
- ✅ ファイル権限管理: 必要に応じてオーナー権限を転送

---

### 4.2 Google Docs - `documents` スコープ

**台本:**
"Next, the app uses the 'documents' permission to add the extracted information - OCR text, key facts, and summaries - to that unified document. This allows the document to become a comprehensive reference for household information."

**アクション:**
1. 作成された Google Doc を開く
2. 新しく追加されたテキストセクションを表示：
   - 日付
   - ファイル名
   - カテゴリ
   - OCR されたテキスト
   - 抽出された重要情報（事実）
   - AI による要約
3. スクロールして複数のエントリを見せる

**スコープの正当性:**
- ✅ ドキュメント編集: OCR テキスト・事実・要約を追記
- ✅ 自動更新: ユーザーが追加処理なしに情報が蓄積

---

### 4.3 Google Calendar - `calendar.events` スコープ

**台本:**
"When the document contains important dates or events - such as school events, appointment deadlines, or payment due dates - the app automatically extracts this information and creates calendar events in Google Calendar."

**アクション:**
1. Google Calendar を開く
2. 処理されたファイルに対応するイベントを表示
3. イベントの詳細を確認：
   - タイトル（例: "学校参観日"、"予防接種予約"）
   - 日時
   - 説明（ファイルの内容を要約したテキスト）

**スコープの正当性:**
- ✅ 予定作成: 抽出された日付から自動カレンダー登録
- ✅ ダブルチェック: 同一日・同一タイトルの重複防止機構を実装

---

### 4.4 Google Tasks - `tasks` スコープ

**台本:**
"Similarly, tasks and deadlines are extracted and automatically added to Google Tasks. The app ensures no duplicate tasks are created even when processing multiple related documents."

**アクション:**
1. Google Tasks を開く
2. 処理されたファイルに対応するタスクを表示
3. タスクの詳細を確認：
   - タイトル（例: "書類提出期限"）
   - 期日
   - 説明

**スコープの正当性:**
- ✅ タスク作成: 抽出された期限からタスク自動登録
- ✅ 重複排除: 同一日同一タイトルで既存タスクとの重複を防止

---

### 4.5 Google Photos - `photoslibrary.appendonly` スコープ

**台本:**
"For documents that are photographs or images, the app automatically uploads them to Google Photos for easy backup and reference."

**アクション:**
1. Google Photos を開く
2. アプリが作成したアルバムまたはアップロード画像を表示
3. 写真が正常にバックアップされたことを確認

**スコープの正当性:**
- ✅ 追記のみ権限: 新しい写真のアップロードのみ
- ✅ 削除権限なし: 既存の写真は削除不可（ユーザーデータ保護）
- ✅ バックアップ機能: 重要な書類写真の自動バックアップ

---

## 5. セキュリティ・プライバシーの説明（1分）

**台本:**
"The app implements several security measures to protect user data:

1. **Token-based authentication**: Admin endpoints are protected with secret tokens stored in Google Cloud Secret Manager.

2. **Scope-specific permissions**: The app only requests permissions needed for its core functionality. It does NOT request full Drive access or email access.

3. **Processed data tracking**: The app marks processed documents with metadata to prevent duplicate processing, even across multiple instances.

4. **OAuth fallback**: If OAuth credentials are unavailable, the app uses a Service Account with limited drive access for file operations only - NOT for sensitive operations like creating Calendar events or uploading Photos.

5. **Privacy**: All document processing happens server-side. The app does NOT store or analyze document content beyond the current session."

**画面:**
- Cloud Console の Secret Manager を表示（説明用）
- ログの構造化ログ機能を表示

---

## 6. ユーザーメリット（30秒）

**台本:**
"By consolidating these permissions, the app provides:

✓ Automatic document organization - no manual filing needed
✓ Calendar and task management - deadlines never missed
✓ Centralized information - all household docs in one unified place
✓ Backup - important documents are backed up to Google Photos
✓ Privacy - full control over data with Google OAuth

All done securely, using only the permissions necessary for these features."

---

## 7. クロージング（15秒）

**台本:**
"This is how HomeDocManager uses each requested permission in alignment with Google's policies. The app respects user privacy, implements robust security, and provides clear value for every permission requested.

Thank you for watching."

---

## 技術的なポイント（撮影時に意識）

### 必須表示要素

1. ✅ OAuth 同意画面（スコープが見える状態）
2. ✅ Google Drive での操作（Inbox → 処理 → 移動）
3. ✅ Google Docs（追記されたテキスト）
4. ✅ Google Calendar（作成されたイベント）
5. ✅ Google Tasks（作成されたタスク）
6. ✅ Google Photos（アップロード）

### NG な撮影方法

- ❌ スコープ同意画面を見せずに進める
- ❌ 処理に失敗した場合のエラーを見せる
- ❌ ユーザーの個人情報を含む画面を撮影
- ❌ 説明なしに処理が進む（ユーザーが何をしているか不明確）

---

## 撮影環境の推奨設定

### 解像度・画質
- 1080p (1920x1080) 以上推奨
- 60fps でも 30fps でも OK

### マイク・音声
- クリアなマイク品質
- バックグラウンドノイズ最小化
- 英語推奨（日本語の場合は英語字幕を付与）

### 時間
- 総時間: 5 〜 7分
- 編集は最小限に（自然な流れが重要）

### アップロード
- YouTube に「限定公開（Unlisted）」でアップロード
- タイトル例: "HomeDocManager - Google Sensitive Scopes Demo"
- 説明欄に以下を記載：
  ```
  This is a demonstration video for Google OAuth Sensitive Scopes verification.
  App: HomeDocManager - Intelligent household document management system
  Scopes used:
  - photoslibrary.appendonly (Google Photos)
  - calendar.events (Google Calendar)
  - tasks (Google Tasks)
  - documents (Google Docs)
  - drive.file (Google Drive)
  ```

---

## 参考リンク

- [Google Sensitive Scopes Verification](https://support.google.com/cloud/answer/9110914)
- [OAuth 2.0 Scopes for Google APIs](https://developers.google.com/identity/protocols/oauth2/scopes)

