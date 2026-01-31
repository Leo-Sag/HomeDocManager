# HomeDocManager Python→Go 移行ガイド

このドキュメントは、既存のPython版HomeDocManagerからGo版への移行手順を説明します。

## 移行戦略

Python版とGo版を並行稼働させ、段階的に移行することを推奨します。

### アプローチ

1. **並行稼働期間**: Python版とGo版を同時に稼働（別々のCloud Runサービス）
2. **検証期間**: Go版で一部のファイルを処理し、結果を検証
3. **完全移行**: 検証が完了したら、トラフィックを完全にGo版に切り替え
4. **Python版廃止**: Go版が安定稼働したら、Python版を削除

## ユーザー側で必要な操作

### ステップ1: 新しいCloud Runサービスのデプロイ

#### 1.1 設定ファイルの編集

```bash
cd cloud-run-go
```

[internal/config/settings.go](internal/config/settings.go) を開き、以下を確認・編集：

- `GCPProjectID`: 現在のプロジェクトID（Python版と同じ）
- `FolderIDs`: Google DriveのフォルダID（**Python版と同じ値を使用**）
- `ChildAliases`, `AdultAliases`: 名寄せルール（必要に応じて更新）
- `GradeConfigSettings`: 学年設定（最新の情報に更新）

#### 1.2 デプロイスクリプトの編集

[deploy.sh](deploy.sh) を開き、`PROJECT_ID` を設定：

```bash
PROJECT_ID="your-actual-project-id"  # 実際のプロジェクトIDに変更
```

#### 1.3 デプロイ実行

```bash
chmod +x deploy.sh
./deploy.sh
```

デプロイが完了すると、新しいCloud RunサービスのURLが表示されます：

```
Service URL: https://homedocmanager-go-xxxxx-an.a.run.app
```

このURLをメモしておいてください。

### ステップ2: Pub/Subの設定（オプション）

Go版専用のPub/Subサブスクリプションを作成する場合：

```bash
# 新しいサブスクリプションを作成
gcloud pubsub subscriptions create homedocmanager-go-sub \
    --topic=drive-events \
    --push-endpoint=https://YOUR_GO_SERVICE_URL/ \
    --ack-deadline=600
```

**注意**: Python版とGo版を同時にテストする場合は、一時的に手動トリガーを使用することを推奨します。

### ステップ3: 動作確認

#### 3.1 ヘルスチェック

```bash
curl https://YOUR_GO_SERVICE_URL/health
```

期待される応答: `{"status":"OK"}`

#### 3.2 テストファイルで動作確認

1. Google Driveの `Inbox` フォルダにテストファイルをアップロード
2. ファイルIDを取得（ブラウザでファイルを開き、URLから取得）
3. 手動トリガーで処理:

```bash
curl -X POST https://YOUR_GO_SERVICE_URL/test \
  -H "Content-Type: application/json" \
  -d '{"file_id": "YOUR_FILE_ID"}'
```

期待される応答:
```json
{
  "status": "success",
  "result": "PROCESSED",
  "file_id": "YOUR_FILE_ID"
}
```

4. Google Driveでファイルが正しく移動・リネームされているか確認

#### 3.3 Inboxの一括処理テスト

```bash
curl -X POST https://YOUR_GO_SERVICE_URL/trigger/inbox
```

Inbox内の全ファイル（最大50件）が処理されます。

### ステップ4: 並行稼働期間の設定

#### 4.1 トラフィック分割（推奨）

一部のトラフィックだけをGo版に流す設定：

```bash
# Cloud Load Balancingを使用してトラフィックを分割
# 例: 10%のトラフィックをGo版に
gcloud run services update-traffic homedocmanager-go \
    --to-revisions=LATEST=10
```

#### 4.2 モニタリング

Cloud Loggingでログを確認：

```bash
# Python版のログ
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager" \
    --limit 20 \
    --format json

# Go版のログ
gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go" \
    --limit 20 \
    --format json
```

### ステップ5: 完全移行

Go版の動作が安定したら、トラフィックを完全に切り替えます。

#### 5.1 Pub/Subサブスクリプションの切り替え

```bash
# Python版のサブスクリプションを停止
gcloud pubsub subscriptions update homedocmanager-sub \
    --push-endpoint=""

# Go版のサブスクリプションを有効化（または新規作成）
gcloud pubsub subscriptions update homedocmanager-go-sub \
    --push-endpoint=https://YOUR_GO_SERVICE_URL/
```

または、既存のサブスクリプションのエンドポイントをGo版に変更：

```bash
gcloud pubsub subscriptions update homedocmanager-sub \
    --push-endpoint=https://YOUR_GO_SERVICE_URL/
```

#### 5.2 動作確認

数日間運用して、以下を確認：
- ファイルが正しく処理されているか
- エラーログがないか
- メモリ使用量、CPU使用率
- レスポンス時間

### ステップ6: Python版の廃止

Go版が安定稼働したら、Python版を削除します。

```bash
# Python版のCloud Runサービスを削除
gcloud run services delete homedocmanager \
    --region asia-northeast1

# 不要になったPub/Subサブスクリプションを削除（Go版に完全移行した場合）
gcloud pubsub subscriptions delete homedocmanager-sub
```

## コスト比較

### 予想されるコスト削減効果

| 項目 | Python版 | Go版 | 削減率 |
|------|----------|------|--------|
| メモリ割当 | 512MB | 256MB | 50% |
| コールドスタート時間 | 2-5秒 | 50-200ms | 95% |
| アイドル時メモリ | ~50MB | ~20MB | 60% |
| CPU効率 | 1.0x | 1.5-2.0x | 50-100% |

### 月間コスト試算（例）

仮定:
- 月間10,000リクエスト
- 平均処理時間: 2秒（外部API待ち時間を含む）
- 同時実行数: 平均5

**Python版（512MB）**:
- メモリコスト: 約$2-3
- CPUコスト: 約$1-2
- 合計: 約$3-5

**Go版（256MB）**:
- メモリコスト: 約$1-1.5
- CPUコスト: 約$0.5-1
- 合計: 約$1.5-2.5

**削減額: 約$1.5-2.5/月（約50%削減）**

## トラブルシューティング

### 問題: ファイルが正しく処理されない

**確認事項:**
1. `internal/config/settings.go` のフォルダIDが正しいか
2. サービスアカウントに適切な権限があるか
3. Secret Managerのシークレットが正しく設定されているか

### 問題: Gemini APIエラー

**確認事項:**
1. Secret Managerに `GEMINI_API_KEY` が登録されているか
2. APIキーが有効か
3. Gemini APIの利用制限に達していないか

### 問題: メモリ不足エラー

**対処法:**
```bash
gcloud run services update homedocmanager-go \
    --memory 512Mi \
    --region asia-northeast1
```

### 問題: タイムアウトエラー

**対処法:**
```bash
gcloud run services update homedocmanager-go \
    --timeout 540 \
    --cpu 2 \
    --region asia-northeast1
```

## ロールバック手順

万が一、Go版に問題が発生した場合のロールバック手順：

```bash
# Pub/SubサブスクリプションをPython版に戻す
gcloud pubsub subscriptions update homedocmanager-sub \
    --push-endpoint=https://YOUR_PYTHON_SERVICE_URL/

# または、Go版のサブスクリプションを無効化
gcloud pubsub subscriptions update homedocmanager-go-sub \
    --push-endpoint=""
```

Python版のCloud Runサービスが削除されている場合は、再デプロイが必要です。

## サポート

問題が発生した場合は、以下を確認してください：
- Cloud Runのログ
- Secret Managerの設定
- サービスアカウントの権限
- Google Drive APIのクォータ

## まとめ

この移行ガイドに従って、段階的にGo版へ移行することで、リスクを最小限に抑えながらパフォーマンスとコスト効率を向上させることができます。

ご不明な点がありましたら、お気軽にお問い合わせください。
