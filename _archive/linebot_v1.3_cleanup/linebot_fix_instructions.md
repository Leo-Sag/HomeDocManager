# HomeDocManager LINE Bot 修正指示（AIエージェント向け）

## 目的

Cloud Run 上で稼働する `internal/linebot` の実装を、以下の要件を満たす形に修正する。

- リッチメニューのトリガー文字列（`__CAT_LIFE__`など）を受信
- カテゴリ別 NotebookLM URL を差し込んだ Flex Message を返す
- Quick Reply でカテゴリ切り替えできる
- 設定ファイルを統合し「一発起動」できる構成にする

---

## 修正タスク一覧

---

## ✅ 1. 統合設定ファイルを新設する

### 新規作成

`resources/linebot/line_settings.json`

### 含める項目

- Flex テンプレパス
- Help テンプレパス
- カテゴリ別 NotebookLM URL
- トリガー文字列一覧
- カテゴリラベル
- 質問例
- Quick Reply 設定（順序含む）

### 例

```json
{
  "flex_template_path": "resources/linebot/line_flex_template.json",
  "help_template_path": "resources/linebot/line_flex_help_message.json",

  "notebooklm_urls": {
    "life": "...",
    "money": "...",
    "children": "...",
    "medical": "...",
    "library": "...",
    "help": "...",
    "default": "..."
  },

  "triggers": {
    "life": "__CAT_LIFE__",
    "money": "__CAT_MONEY__",
    "children": "__CAT_CHILDREN__",
    "medical": "__CAT_MEDICAL__",
    "library": "__LIBRARY__",
    "help": "__HELP__"
  },

  "category_labels": {
    "life": "🏠 生活",
    "money": "💰 お金",
    "children": "👶 子供",
    "medical": "🏥 医療",
    "library": "📚 ライブラリ",
    "help": "❓ 使い方"
  },

  "examples": {
    "life": ["手続きの期限は？", "必要書類は何？"],
    "money": ["医療費控除はいくら？", "保険料の支払いは？"]
  },

  "quick_reply": {
    "enabled": true,
    "include_current": true,
    "current_prefix": "✅ ",
    "order": ["life", "money", "children", "medical", "library", "help"]
  }
}
```

---

## ✅ 2. Settings 構造体を統合JSON仕様に拡張する

### 修正対象

`internal/linebot/service.go`

### 修正内容

```go
type QuickReplyConfig struct {
    Enabled        bool     `json:"enabled"`
    IncludeCurrent bool     `json:"include_current"`
    CurrentPrefix  string   `json:"current_prefix"`
    Order          []string `json:"order"`
}

type Settings struct {
    FlexTemplatePath string `json:"flex_template_path"`
    HelpTemplatePath string `json:"help_template_path"`

    NotebookLMURLs  map[string]string   `json:"notebooklm_urls"`
    Triggers        map[string]string   `json:"triggers"`
    CategoryLabels  map[string]string   `json:"category_labels"`
    Examples        map[string][]string `json:"examples"`
    QuickReply      QuickReplyConfig    `json:"quick_reply"`
}
```

---

## ✅ 3. NewService を「settingsPathだけ」で起動できる形にする

### 修正前

```go
NewService(settingsPath, templatePath, helpPath string)
```

### 修正後

```go
NewService(settingsPath string)
```

テンプレは settings 内のパスから読む：

```go
t := loadTemplate(s.FlexTemplatePath)
h := loadTemplate(s.HelpTemplatePath)
```

---

## ✅ 4. BuildFlexMessage をカテゴリ別URL対応にする

### 修正内容

- NotebookLM URL をカテゴリで切り替える

```go
url := s.settings.NotebookLMURLs[category]
if url == "" {
    url = s.settings.NotebookLMURLs["default"]
}
```

---

## ✅ 5. ✅マークは Quick Reply 側で表示する

### 修正内容

- Flexタイトルに固定で✅を付けない

```go
title := label
```

---

## ✅ 6. Quick Reply を currentカテゴリ付きで生成する

### 修正

```go
GetQuickReplyItems(current string)
```

### currentカテゴリだけ prefix を付ける

```go
if cat == current {
    label = "✅ " + label
}
```

---

## ✅ 7. handler.go を current対応にする

### 修正前

```go
flexContents, _ := BuildFlexMessage(text)
quick := GetQuickReplyItems()
```

### 修正後

```go
category, flexContents, _ := BuildFlexMessage(text)
quick := GetQuickReplyItems(category)
```

---

## ✅ 8. main.go を統合設定1本で起動

### 修正前

```go
NewService(settingsPath, flexPath, helpPath)
```

### 修正後

```go
NewService(config.LineBotSettingsPath)
```

---

## ✅ 9. config 側の環境変数整理

### 残す

```go
LINE_BOT_SETTINGS_PATH
```

### 削除（不要）

- LINE_FLEX_TEMPLATE_PATH
- LINE_FLEX_HELP_PATH

---

## 完了条件（Definition of Done）

- `/callback` Webhook が動作する
- カテゴリ別に NotebookLM URL が切り替わる
- Quick Reply でカテゴリ切り替えできる
- 設定ファイル1つで起動できる

---
