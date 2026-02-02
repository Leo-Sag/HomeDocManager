# ✅ 修正案B（完全版）：Flexテンプレのラッパー対応を handler.go 側で吸収する

このドキュメントは、LINE Bot のカテゴリ返信が動かない原因を解決するための  
**修正案B（コード側でFlexラッパーを吸収する方法）**をまとめたものです。

---

# ✅ 背景：なぜ help だけ動いて他カテゴリが落ちるのか？

現在の実装では：

- `help` は返信できる  
- `life / money / children ...` は返信されない

という状態になっています。

これはテンプレJSONの構造差が原因です。

---

## ✅ helpテンプレ（動く）

`line_flex_help_message.json` は bubble本体だけです：

```json
{
  "type": "bubble",
  ...
}
```

これは SDK の `UnmarshalFlexMessageJSON()` が期待する形式です。

---

## ❌ 通常カテゴリテンプレ（落ちる）

`line_flex_template.json` は Flex Message全体（ラッパー付き）になっています：

```json
{
  "type": "flex",
  "altText": "案内",
  "contents": {
    "type": "bubble",
    ...
  }
}
```

この形式をそのまま `UnmarshalFlexMessageJSON()` に渡すと失敗します。

---

# ✅ 修正案B：handler.go 側で contents を吸収する

テンプレJSONを変更せずに、コード側で

- altText を拾う
- contents があれば bubble 本体だけ取り出す

ようにします。

これで **helpも通常カテゴリも両方動きます。**

---

# ✅ 修正手順

---

## Step 1：handleTextMessage 内で payload を正規化する

`handler.go` の `handleTextMessage()` に以下を追加します。

### ✅ 修正版コード

```go
func (h *Handler) handleTextMessage(replyToken, text string) {
    // Flex Messageを生成
    category, flexContents, err := h.service.BuildFlexMessage(text)
    if err != nil {
        log.Printf("Error building flex message: %v", err)
        return
    }

    // ✅ altText と payload を正規化
    altText := "NotebookLM案内"
    payload := flexContents

    // テンプレに altText があれば採用
    if v, ok := flexContents["altText"].(string); ok && v != "" {
        altText = v
    }

    // ✅ contents ラッパーがある場合は bubble 本体だけ抜く
    if cAny, ok := flexContents["contents"]; ok {
        if cMap, ok := cAny.(map[string]interface{}); ok {
            payload = cMap
        }
    }

    // ✅ bubble本体だけを Unmarshal する
    b, err := json.Marshal(payload)
    if err != nil {
        log.Printf("Error marshaling flex message: %v", err)
        return
    }

    container, err := linebot.UnmarshalFlexMessageJSON(b)
    if err != nil {
        log.Printf("Error unmarshaling flex message: %v", err)
        return
    }

    // ✅ altText を反映して返信する
    msg := linebot.NewFlexMessage(altText, container)

    // Quick Replyを追加（既存処理はそのまま）
    quickReplyItems := h.service.GetQuickReplyItems(category)

    var qrItems []*linebot.QuickReplyButton
    for _, item := range quickReplyItems {
        actionAny, ok := item["action"]
        if !ok {
            continue
        }
        action, ok := actionAny.(map[string]interface{})
        if !ok {
            continue
        }

        label, _ := action["label"].(string)
        text, _ := action["text"].(string)

        if label == "" || text == "" {
            continue
        }

        qrItems = append(qrItems,
            linebot.NewQuickReplyButton(
                "",
                linebot.NewMessageAction(label, text),
            ),
        )
    }

    if len(qrItems) > 0 {
        msg.WithQuickReplies(linebot.NewQuickReplyItems(qrItems...))
    }

    // ✅ Reply送信
    res := h.bot.ReplyMessage(replyToken, msg)
    if _, err := res.Do(); err != nil {
        log.Printf("Error replying message: %v", err)
    }
}
```

---

# ✅ 修正後の効果

| 項目 | 結果 |
|------|------|
| helpテンプレ（bubble形式） | ✅ 動く |
| 通常テンプレ（flexラッパー付き） | ✅ 動く |
| altTextもテンプレ側で管理できる | ✅ |
| テンプレ形式の揺れに強くなる | ✅ |

---

# ✅ 推奨

短期的にはこの修正案Bが最も安全です。

将来的にはテンプレJSONを全て bubble形式に統一する（修正案A）と  
よりシンプルになります。

---

# ✅ 最後に：確認方法

Cloud Run ログで以下が出なくなれば成功です：

- Error unmarshaling flex message
- ReplyMessage failed

カテゴリボタンを押して Flex が返ればOK ✅

---
