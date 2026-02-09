# Cloud Run ログ確認コマンド集

デモ動画撮影時に使用するログ確認コマンドをまとめました。

---

## 1. 最新ログをリアルタイム表示（デモに最適）

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go" \
  --project=family-document-manager-486009 \
  --limit=50 \
  --format="table(timestamp,jsonPayload.message)" \
  --sort-by=~timestamp
```

**説明**: 最新 50 件のログをタイムスタンプとメッセージで表示

---

## 2. ファイル処理の全ステップを追う

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"Processing file\"" \
  --project=family-document-manager-486009 \
  --limit=20 \
  --format="table(timestamp,jsonPayload.message)"
```

**説明**: ファイル処理開始ログのみを抽出

---

## 3. 各処理ステップの詳細ログ

### ファイル検知
```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"Change detected\"" \
  --project=family-document-manager-486009 \
  --limit=10
```

### OCR 実行
```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"OCR\"" \
  --project=family-document-manager-486009 \
  --limit=10
```

### Gemini 分類実行
```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"classified\"" \
  --project=family-document-manager-486009 \
  --limit=10
```

### ファイル移動
```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"moved\"" \
  --project=family-document-manager-486009 \
  --limit=10
```

---

## 4. Google Calendar イベント作成の確認

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"calendar\"" \
  --project=family-document-manager-486009 \
  --limit=10 \
  --format="table(timestamp,jsonPayload.message)"
```

---

## 5. Google Tasks タスク作成の確認

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"task\"" \
  --project=family-document-manager-486009 \
  --limit=10 \
  --format="table(timestamp,jsonPayload.message)"
```

---

## 6. Google Docs 追記の確認

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"NotebookLM\"" \
  --project=family-document-manager-486009 \
  --limit=10 \
  --format="table(timestamp,jsonPayload.message)"
```

---

## 7. Google Photos アップロードの確認

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"Photos\"" \
  --project=family-document-manager-486009 \
  --limit=10 \
  --format="table(timestamp,jsonPayload.message)"
```

---

## 8. Watch 状態確認

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"Watch\"" \
  --project=family-document-manager-486009 \
  --limit=10 \
  --format="table(timestamp,jsonPayload.message)"
```

---

## 9. エラーのみを表示

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND severity=ERROR" \
  --project=family-document-manager-486009 \
  --limit=20 \
  --format="table(timestamp,jsonPayload.message,severity)"
```

---

## 10. 特定のファイルID処理を追跡

```bash
# {FILE_ID} をコピーしたファイル ID に置き換え
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"{FILE_ID}\"" \
  --project=family-document-manager-486009 \
  --limit=50 \
  --format="table(timestamp,jsonPayload.message)"
```

---

## デモ動画撮影時の使用手順

### ステップ 1: ターミナルを 2 つ開く

1. **ターミナル A**: ログ監視用（コマンド #1 を実行）
2. **ターミナル B**: ファイルアップロード用

### ステップ 2: ログ監視開始

```bash
# ターミナル A で実行
watch -n 1 'gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go" --project=family-document-manager-486009 --limit=20 --format="table(timestamp,jsonPayload.message)" --sort-by=~timestamp'
```

### ステップ 3: ファイルをアップロード

```bash
# ターミナル B で実行（または Google Drive UI で手動アップロード）
```

### ステップ 4: ログをリアルタイム監視

ログが自動更新される様子をスクリーンキャプチャ

---

## より詳細なログ情報を表示

### JSON 形式で完全なメタデータを表示

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go" \
  --project=family-document-manager-486009 \
  --limit=5 \
  --format=json | jq '.'
```

### タイムスタンプ + メッセージ + ログレベルを表示

```bash
gcloud logging read \
  "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go" \
  --project=family-document-manager-486009 \
  --limit=30 \
  --format="table(timestamp.format('%Y-%m-%d %H:%M:%S'),severity,jsonPayload.message)"
```

---

## 便利なエイリアス設定（`.bashrc` または `.zshrc` に追加）

```bash
# HomeDocManager ログの簡易表示
alias hdm-logs='gcloud logging read "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go" --project=family-document-manager-486009 --limit=50 --format="table(timestamp,jsonPayload.message)" --sort-by=~timestamp'

# 特定キーワード検索
hdm-search() {
  gcloud logging read \
    "resource.type=cloud_run_revision AND resource.labels.service_name=homedocmanager-go AND jsonPayload.message:\"$1\"" \
    --project=family-document-manager-486009 \
    --limit=20 \
    --format="table(timestamp,jsonPayload.message)"
}

# 使用例: hdm-search "calendar"
```

`.bashrc` に追加した場合:
```bash
source ~/.bashrc
hdm-logs                    # 最新ログ表示
hdm-search "calendar"       # "calendar" 含むログを検索
```

---

## 期待されるログ出力例

### ファイル処理の完全な流れ

```
2026-02-09 02:45:30  Change detected: 1a2b3c4d5e6f (school_notice.pdf)
2026-02-09 02:45:31  Processing file from notification: 1a2b3c4d5e6f
2026-02-09 02:45:32  OCR completed: 123 characters extracted
2026-02-09 02:45:33  File classified: 40_子供・教育/01_お便り
2026-02-09 02:45:34  File renamed: 20260209_学校参観日_お知らせ.pdf
2026-02-09 02:45:35  File moved to: 40_子供・教育/01_お便り/2026
2026-02-09 02:45:36  Calendar event created: 学校参観日 (2026-03-15)
2026-02-09 02:45:37  Task created: 学校参観日_03月15日
2026-02-09 02:45:38  NotebookLM同期完了: 20260209_学校参観日_お知らせ.pdf → 2026年度_子供・教育
```

---

## トラブルシューティング

### ログが表示されない場合

1. プロジェクト ID が正しいか確認
```bash
gcloud config get-value project
```

2. Cloud Run サービスが実行中か確認
```bash
gcloud run services describe homedocmanager-go --region=asia-northeast1 --project=family-document-manager-486009
```

3. ファイルが実際に処理されているか確認（管理画面）
```bash
ADMIN_TOKEN=$(gcloud secrets versions access latest --secret=ADMIN_TOKEN --project=family-document-manager-486009)
curl -H "Authorization: Bearer $ADMIN_TOKEN" https://homedocmanager-go-p6nqlr6e4a-an.a.run.app/admin/info | jq '.'
```

---

## 参考: Cloud Logging フィルター構文

| 検索条件 | 例 |
|---------|-----|
| 特定メッセージ | `jsonPayload.message:"calendar"` |
| 複数キーワード AND | `jsonPayload.message:"calendar" AND jsonPayload.message:"created"` |
| 複数キーワード OR | `jsonPayload.message:"calendar" OR jsonPayload.message:"task"` |
| 否定 | `NOT jsonPayload.message:"error"` |
| ログレベル | `severity=ERROR` または `severity=INFO` |
| 時間範囲 | `timestamp>="2026-02-09T00:00:00Z" AND timestamp<"2026-02-09T10:00:00Z"` |

