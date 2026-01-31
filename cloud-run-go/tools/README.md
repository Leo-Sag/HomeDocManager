# OAuth 2.0 セットアップツール

このツールは、Google Photos、Calendar、Tasks APIへのアクセスに必要なリフレッシュトークンを取得します。

## 前提条件

1. Go 1.23以上がインストールされていること
2. GCP Console でOAuthクライアントIDを作成済みであること
3. `client_secret.json` をダウンロード済みであること

## 使用方法

### 1. client_secret.json を配置

ダウンロードしたOAuthクライアントのJSONファイルを、`cloud-run-go` ディレクトリに配置：

```bash
cd HomeDocManager/cloud-run-go
# client_secret.json をここに配置
ls client_secret.json  # 確認
```

### 2. 環境変数を設定

```bash
export GCP_PROJECT_ID="your-actual-project-id"
```

### 3. 依存関係をダウンロード

```bash
go mod download
```

### 4. スクリプトを実行

```bash
go run tools/setup_oauth.go
```

### 5. ブラウザで認証

スクリプトを実行すると：

1. 自動的にブラウザが開きます
2. Googleアカウントでログイン
3. 以下の権限を許可：
   - ✅ Google Photosへの写真の追加
   - ✅ Googleカレンダーの読み書き
   - ✅ Google Tasksの読み書き
4. 「許可」をクリック
5. 「認証成功！」と表示されたらターミナルに戻る

### 6. Secret Managerに保存

```
Secret Managerに保存しますか? (y/n): y
```

`y` を入力すると、自動的にSecret Managerに保存されます。

## 出力ファイル

- `token.json` - 取得したトークン全体（参考用）

## トラブルシューティング

### エラー: `client_secret.json not found`

`client_secret.json` が `cloud-run-go` ディレクトリにあることを確認してください。

### エラー: `GCP_PROJECT_ID environment variable is not set`

環境変数を設定してください：

```bash
export GCP_PROJECT_ID="your-project-id"
```

### ブラウザが自動で開かない場合

表示されたURLを手動でブラウザにコピー&ペーストしてください。

### ポート8080が使用中の場合

`setup_oauth.go` の `Addr: ":8080"` を別のポート（例：`:8081`）に変更してください。
同時に `http://localhost:8081/oauth2callback` をOAuthクライアントのリダイレクトURIに追加してください。

### Error: No refresh token returned

これは、既に一度認証済みの場合に発生します。以下の手順で解決：

1. [Google Account 権限管理](https://myaccount.google.com/permissions) を開く
2. HomeDocManagerアプリのアクセスを取り消す
3. もう一度 `go run tools/setup_oauth.go` を実行

## 確認方法

Secret Managerに正しく保存されたか確認：

```bash
gcloud secrets versions access latest --secret="OAUTH_REFRESH_TOKEN"
```

## 次のステップ

リフレッシュトークンの取得が完了したら：

1. `deploy.sh` を編集して `PROJECT_ID` を設定
2. `./deploy.sh` を実行してCloud Runにデプロイ
