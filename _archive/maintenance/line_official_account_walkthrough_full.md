# HomeDocManager LINE公式アカウント設定ウォークスルー（完全版）

このドキュメントは **HomeDocManager × NotebookLM × LINE入口Bot** を  
LINE公式アカウント側で本番運用できる状態にするための手順です。

目的：

- Cloud Run `/callback` Webhook をLINEに接続  
- リッチメニューをJSONで一括登録  
- ボタン押下 → トリガー送信 → Flex返信 → NotebookLMを開く

---

# ✅ 前提（すでに完了していること）

- Cloud Run にBotがデプロイ済み  
- `/callback` が実装されている  
- `line_settings.json` が統合済み  
- Flex Message返信が動作する  

---

# 1. LINE Developers チャネル準備

---

## ① LINE Developers にログイン

https://developers.line.biz/

Provider → HomeDocManager Bot を選択

---

## ② Messaging APIチャネルを作成

- チャネル作成 → Messaging API
- チャネル名：HomeDocManager Bot
- 説明：家庭書類検索Bot

---

## ③ Channel Secret / Access Token を取得

---

### Channel Secret

Messaging API → Basic settings  
→ Channel secret をコピー

---

### Channel Access Token（長期）

Messaging API → 下部 → Channel access token  
→ Issue を押して発行

---

# 2. Cloud Run に環境変数をセット

Cloud Run サービスに以下を登録：

| Key | Value |
|-----|------|
| LINE_CHANNEL_SECRET | Channel secret |
| LINE_CHANNEL_ACCESS_TOKEN | Channel access token |
| LINE_BOT_SETTINGS_PATH | resources/linebot/line_settings.json |

---

# 3. Webhook URL を設定する

---

## Cloud Run URL を確認

例：

```
https://xxxxx.run.app
```

Webhook URL：

```
https://xxxxx.run.app/callback
```

---

## LINE Developers 設定

Messaging API → Webhook settings

- Webhook URL を入力  
- Use webhook → ON  
- Verify ボタン → Success を確認

---

# 4. LINE公式アカウント側の応答設定（重要）

LINE Official Account Manager：

https://manager.line.biz/

---

### 応答設定

| 設定 | 推奨 |
|------|------|
| 応答メッセージ | OFF |
| あいさつメッセージ | OFF |
| Webhook | ON |

※これをしないと二重返信になります。

---

# 5. リッチメニューをJSONで一括登録する

---

## ✅ 必要ファイル

- richmenu JSON  
- richmenu PNG画像（2500×1686推奨）

生成済み：

- `richmenu_homedocmanager_6buttons.json`
- `richmenu_homedocmanager_6buttons.png`

---

## ① 環境変数をセット

```bash
export LINE_CHANNEL_ACCESS_TOKEN="YOUR_LONG_LIVED_TOKEN"
export LINE_API="https://api.line.me"
```

---

## ② リッチメニュー作成

```bash
curl -sS -X POST "$LINE_API/v2/bot/richmenu" \
  -H "Authorization: Bearer $LINE_CHANNEL_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d @richmenu_homedocmanager_6buttons.json
```

成功すると返ります：

```json
{"richMenuId":"richmenu-xxxxxxxxxxxx"}
```

保存：

```bash
export RICHMENU_ID="richmenu-xxxxxxxxxxxx"
```

---

## ③ 画像をアップロード

```bash
curl -sS -X POST "$LINE_API/v2/bot/richmenu/$RICHMENU_ID/content" \
  -H "Authorization: Bearer $LINE_CHANNEL_ACCESS_TOKEN" \
  -H "Content-Type: image/png" \
  --data-binary @richmenu_homedocmanager_6buttons.png
```

---

## ④ 全ユーザーに適用

```bash
curl -sS -X POST "$LINE_API/v2/bot/user/all/richmenu/$RICHMENU_ID" \
  -H "Authorization: Bearer $LINE_CHANNEL_ACCESS_TOKEN"
```

---

# 6. 動作確認

Botを開いて確認：

- リッチメニューが表示される  
- ボタンを押すとトリガー文字列が送信される  

例：

| ボタン | 送信文字列 |
|------|-----------|
| 🏠生活 | __CAT_LIFE__ |
| 💰お金 | __CAT_MONEY__ |
| 👶子供 | __CAT_CHILDREN__ |
| 🏥医療 | __CAT_MEDICAL__ |
| 📚ライブラリ | __LIBRARY__ |
| ❓迷ったら | __HELP__ |

BotからFlex Messageが返れば成功 ✅

---

# 7. 既存リッチメニュー管理（任意）

---

### 一覧取得

```bash
curl -sS "$LINE_API/v2/bot/richmenu/list" \
  -H "Authorization: Bearer $LINE_CHANNEL_ACCESS_TOKEN" | jq
```

---

### 削除

```bash
curl -sS -X DELETE "$LINE_API/v2/bot/richmenu/RICHMENU_ID" \
  -H "Authorization: Bearer $LINE_CHANNEL_ACCESS_TOKEN"
```

---

# ✅ 完成

これで家族は：

✅ リッチメニューを押す  
✅ Flexで案内が返る  
✅ NotebookLMを開く  
✅ 家庭書類を質問できる  

状態になります。

---
