package linebot

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

type QuickReplyConfig struct {
	Enabled        bool     `json:"enabled"`
	IncludeCurrent bool     `json:"include_current"`
	CurrentPrefix  string   `json:"current_prefix"`
	Order          []string `json:"order"`
}

type Settings struct {
	FlexTemplatePath   string              `json:"flex_template_path"`
	HelpTemplatePath   string              `json:"help_template_path"`
	AITipsTemplatePath string              `json:"ai_tips_template_path"`
	NotebookLMURLs     map[string]string   `json:"notebooklm_urls"`
	Triggers           map[string]string   `json:"triggers"`
	CategoryLabels     map[string]string   `json:"category_labels"`
	Examples           map[string][]string `json:"examples"`
	QuickReply         QuickReplyConfig    `json:"quick_reply"`
}

type FlexTemplate struct {
	raw map[string]interface{}
}

type Service struct {
	settings       *Settings
	template       *FlexTemplate
	helpTemplate   *FlexTemplate
	aiTipsTemplate *FlexTemplate
	mu             sync.RWMutex
}

func NewService(settingsPath string) (*Service, error) {
	s, err := loadSettings(settingsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load settings: %w", err)
	}

	t, err := loadTemplate(s.FlexTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load template from %s: %w", s.FlexTemplatePath, err)
	}

	h, err := loadTemplate(s.HelpTemplatePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load help template from %s: %w", s.HelpTemplatePath, err)
	}

	a, err := loadTemplate(s.AITipsTemplatePath)
	if err != nil {
		// AI Tips テンプレートがない場合は警告のみ（後方互換性）
		log.Printf("Warning: ai_tips_template_path not found or failed to load: %v", err)
	}

	return &Service{
		settings:       s,
		template:       t,
		helpTemplate:   h,
		aiTipsTemplate: a,
	}, nil
}

func loadSettings(path string) (*Settings, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func loadTemplate(path string) (*FlexTemplate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	return &FlexTemplate{raw: raw}, nil
}

// buildGuideText は家族向けの「迷ったらOK」＋「質問のコツ」を返す
func buildGuideText() string {
	// Flexのtextは wrap:true なので改行を入れても見やすい
	return "📌 迷ったらそのまま質問してOKです。\n✅ コツ：質問の最初に「生活：」「医療：」など付けると探しやすいです。"
}

// BuildFlexMessage はトリガー文字列を元にカテゴリ特定とFlex Messageのコンテンツを生成
func (s *Service) BuildFlexMessage(trigger string) (string, map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	category := "unknown"
	for cat, t := range s.settings.Triggers {
		if t == trigger {
			category = cat
			break
		}
	}

	url := s.settings.NotebookLMURLs[category]
	if url == "" {
		url = s.settings.NotebookLMURLs["default"]
	}

	// 使い方(help)の場合は専用テンプレ
	if category == "help" || trigger == "__HELP__" {
		contents, err := s.helpTemplate.build(map[string]string{
			"NOTEBOOKLM_URL": url,
		})
		return category, contents, err
	}

	// AI Tipsの場合は専用テンプレ
	if category == "aitips" || trigger == "__AI_TIPS__" {
		if s.aiTipsTemplate != nil {
			contents, err := s.aiTipsTemplate.build(nil)
			return category, contents, err
		}
	}

	label := s.settings.CategoryLabels[category]
	if label == "" {
		label = category
	}
	title := label
	desc := fmt.Sprintf("%sの書類を調べられます。\n%s", label, buildGuideText())

	if category == "unknown" {
		title = "使い方・カテゴリ選択"
		desc = "迷ったらそのまま質問してOKです。\n下のカテゴリボタン（またはリッチメニュー）から選んでも探せます。\n" + buildGuideText()
	}

	examples := s.settings.Examples[category]
	ex1, ex2 := "質問を入力してください", "例：この書類の期限は？"
	if len(examples) >= 2 {
		ex1 = examples[0]
		ex2 = examples[1]
	} else if len(examples) == 1 {
		ex1 = examples[0]
	}

	vars := map[string]string{
		"TITLE":          title,
		"SUBTITLE":       desc,
		"NOTEBOOKLM_URL": url,
		"EXAMPLE_1":      ex1,
		"EXAMPLE_2":      ex2,
	}

	contents, err := s.template.build(vars)
	return category, contents, err
}

func (ft *FlexTemplate) build(vars map[string]string) (map[string]interface{}, error) {
	b, err := json.Marshal(ft.raw)
	if err != nil {
		return nil, err
	}
	s := string(b)
	for k, v := range vars {
		// JSON文字列として安全にエスケープする
		// 改行やダブルクォートが含まれてもJSONを壊さない
		esc, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		// esc は `"...."` の形なので外側の " を取り除く
		// 例: "a\nb" → a\\nb
		escapedValue := string(esc[1 : len(esc)-1])

		s = strings.ReplaceAll(s, "{{"+k+"}}", escapedValue)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) GetQuickReplyItems(current string) []map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.settings.QuickReply.Enabled {
		return nil
	}

	var items []map[string]interface{}
	order := s.settings.QuickReply.Order
	if len(order) == 0 {
		order = []string{"life", "money", "children", "medical", "library", "help"}
	}

	for _, cat := range order {
		trigger, ok := s.settings.Triggers[cat]
		if !ok {
			continue
		}
		label := s.settings.CategoryLabels[cat]
		if label == "" {
			label = cat
		}

		// カレントカテゴリに ✅ を付記
		if cat == current && s.settings.QuickReply.IncludeCurrent {
			prefix := s.settings.QuickReply.CurrentPrefix
			if prefix == "" {
				prefix = "✅ "
			}
			label = prefix + label
		}

		items = append(items, map[string]interface{}{
			"type": "action",
			"action": map[string]interface{}{
				"type":  "message",
				"label": label,
				"text":  trigger,
			},
		})
	}
	return items
}

// IsTriggerWord はテキストがトリガーワードに一致するかを判定
// トリガーワードの場合はFlex Messageモード、それ以外はRAGモードで処理
func (s *Service) IsTriggerWord(text string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 特殊な内部トリガー（リッチメニュー用）
	if text == "__HELP__" || text == "__AI_TIPS__" {
		return true
	}

	for _, trigger := range s.settings.Triggers {
		if trigger == text {
			return true
		}
	}
	return false
}
