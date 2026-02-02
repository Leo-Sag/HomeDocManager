# server struct にテンプレを持たせる
package main

import (
	"encoding/json"
	"os"
	"strings"
)

type FlexTemplate struct {
	raw map[string]interface{}
}

func LoadFlexTemplate(path string) (*FlexTemplate, error) {
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

func (ft *FlexTemplate) Build(vars map[string]string) (map[string]interface{}, error) {
	// JSONを文字列化してまとめて置換する
	b, err := json.Marshal(ft.raw)
	if err != nil {
		return nil, err
	}
	s := string(b)

	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}

	// 差し替え後に 다시JSON化
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(s), &result); err != nil {
		return nil, err
	}

	return result, nil
}


# カテゴリ別の文言・例文を組み立てる
func buildVars(s *Settings, cat Category) map[string]string {
	key := string(cat)
	if _, ok := s.Messages[key]; !ok {
		key = "unknown"
	}

	title := s.Messages[key].Title
	sub := s.Messages[key].Desc
	url := s.NotebookLM[key]

	// 質問例（カテゴリごと）
	ex1 := "この書類の提出期限は？"
	ex2 := "重要な注意点をまとめて"

	switch cat {
	case CatMoney:
		ex1 = "今年の医療費控除はいくら？"
		ex2 = "保険料の支払い内容を教えて"
	case CatChildren:
		ex1 = "学校からの提出物は何？"
		ex2 = "支援制度の条件は？"
	case CatMedical:
		ex1 = "薬の服用注意点は？"
		ex2 = "診療費の内訳は？"
	}

	return map[string]string{
		"TITLE":          title,
		"SUBTITLE":       sub,
		"NOTEBOOKLM_URL": url,
		"EXAMPLE_1":      ex1,
		"EXAMPLE_2":      ex2,
	}
}
