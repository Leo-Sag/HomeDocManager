# HomeDocManager (Smart Document Filing System) v1.2.0

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

### 3. NotebookLM 同期機能 (ENHANCED)

- **カテゴリ別分割同期**: 以下の 6 つのカテゴリ別に Google Docs (統合ドキュメント) を自動作成・追記します。
  - `life` (30_ライフ・行政)
  - `money` (10_マネー・税務)
  - `children` (40_子供・教育)
  - `medical` (60_ヘルス・医療)
  - `library` (90_ライブラリ)
  - `assets` (20_プロジェクト・資産)
- **OCRBundle 導入**: プレーンなテキストだけでなく、重要な事実(Facts)や要約(Summary)を構造化データとして抽出・蓄積。
- **重複防止**: Mutex による排他制御と `EndOfSegmentLocation` を使用した安定した追記処理。
- **共有ドライブ対応**: 共有ドライブ上のドキュメントも同期対象としてサポート。

### 4. 写真・画像管理

- **Google フォト同期**: 領収書や写真画像を Google フォトへ自動アップロード。
- **高画質変換**: PDFから画像への変換時、Google フォト用には **300 DPI** を使用し、鮮明な画質で保存。
- **トークン最適化**: Gemini解析用には、PDFをネイティブ形式（または200 DPI）で処理することで、APIのトークン消費を抑制。

### 5. 管理・運用ツール

- **ヘルスチェック**: `/health` エンドポイントによる動作確認。
- **インボックス強制スキャン**: `/trigger/inbox` により、Inbox 内のファイルを一括手動処理。
- **ストレージ容量解決**: ユーザーの OAuth トークンを使用することで、サービスアカウントの容量制限を回避。

## システム構成

- **言語**: Go 1.24
- **プラットフォーム**: Google Cloud Run
- **インフラ**: Google Pub/Sub (Drive変更通知), Secret Manager (認証情報管理)

## ディレクトリ構造

```text
HomeDocManager/
├── cloud-run-go/           # Goアプリケーション本体 (v1.2.0 Core)
│   ├── cmd/                # エントリポイント (server)
│   ├── internal/           # 内部ロジック (service, handler, config, model)
│   ├── tools/              # 運用ツール (setup_oauth, etc.)
│   └── Dockerfile          # コンテナ定義
├── _archive/               # 過去の遺産コード
├── README.md               # 本ファイル
└── ...
```

## セットアップ

1. **認証設定**:
   - `cloud-run-go/internal/service/auth_helper.go` を通じて Google API の認証を行います。
   - 取得したリフレッシュトークンを Secret Manager の `OAUTH_REFRESH_TOKEN` に設定します。

2. **デプロイ**:
   - `gcloud run deploy` コマンドを使用して Cloud Run にデプロイします。

## ライセンス

Private Use Only
