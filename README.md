# HomeDocManager (Smart Document Filing System) v1.0.0

Gemini AIを活用して、Googleドライブ上のドキュメント（PDF/画像）を自動で解析・リネーム・分別するシステムです。
Go言語によるリファクタリングにより、高速かつ堅牢な処理を実現しました。

## 主な機能

### 1. インテリジェントなファイル自動仕分け

- **自動解析**: Gemini 1.5 Flash/Pro を使用し、内容を詳細に解析。
- **リネーム・移動**: 解析結果に基づき、適切なファイル名に変更し、年度・カテゴリ別のフォルダへ自動移動。
- **Inbox限定処理**: `00_Inbox` フォルダに入った新規ファイルのみを処理対象とし、意図しない再移動（無限ループ）を防止。

### 2. カレンダー・タスク連携

- **行事抽出**: プリント等の内容から日付・行事名を抽出し、Google カレンダーへ登録。
- **タスク登録**: 提出期限などを抽出し、Google Tasks へ登録。
- **タスクマージ**: 同一日の同じ内容のタスクを自動的に 1 つにマージして登録（ granular な登録を防止）。
- **児童特定優先ロジック**: 複数児童が含まれるフォルダでも、OCR 結果（学年等）から最適な児童を自動特定。

### 3. NotebookLM 同期機能

- **Markdown生成**: 解析結果を Markdown 形式で統合ドキュメントに蓄積。
- **重複防止**: Mutex による排他制御を導入し、統合ドキュメントの重複作成を防止。
- **共有ドライブ対応**: 共有ドライブ上のドキュメントも同期対象としてサポート。

### 4. 写真・画像管理

- **Google フォト同期**: 領収書や写真画像を Google フォトへ自動アップロード。

### 5. 管理・運用ツール

- **ヘルスチェック**: `/health` エンドポイントによる動作確認。
- **インボックス強制スキャン**: `/trigger/inbox` により、Inbox 内のファイルを一括手動処理。
- **ストレージ容量解決**: ユーザーの OAuth トークンを使用することで、サービスアカウントの容量制限を回避。

## システム構成

- **言語**: Go 1.23
- **プラットフォーム**: Google Cloud Run
- **インフラ**: Google Pub/Sub (Drive変更通知), Secret Manager (認証情報管理)

## ディレクトリ構造

```
HomeDocManager/
├── cloud-run-go/           # Goアプリケーション本体 (v1.0.0 Core)
│   ├── cmd/                # エントリポイント (server)
│   ├── internal/           # 内部ロジック (service, handler, config, model)
│   ├── tools/              # 運用ツール (setup_oauth, etc.)
│   └── Dockerfile          # コンテナ定義
├── _archive/               # 過去の遺産コード
│   ├── python_v0/          # 旧Python版
│   └── maintenance/        # メンテナンス用一時ファイル
└── README.md               # 本ファイル
```

## セットアップ

1. **認証設定**:
   - `cloud-run-go/tools/setup_oauth.go` を実行し、ブラウザで Google Drive/Photos/Calendar/Tasks の権限を許可します。
   - 取得したリフレッシュトークンを Secret Manager の `OAUTH_REFRESH_TOKEN` に設定します。

2. **デプロイ**:
   - `sh deploy-cloudbuild.sh` を実行して Cloud Run にデプロイします。

## ライセンス

Private Use Only
