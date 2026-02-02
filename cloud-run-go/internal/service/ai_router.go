package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/google/generative-ai-go/genai"
	"github.com/leo-sagawa/homedocmanager/internal/config"
	"github.com/leo-sagawa/homedocmanager/internal/model"
	"google.golang.org/api/option"
)

// AIRouter はGemini Flash/Proを使い分けるAIルーター
type AIRouter struct {
	apiKey string
	client *genai.Client
}

// NewAIRouter は新しいAIRouterを作成
func NewAIRouter(ctx context.Context) (*AIRouter, error) {
	apiKey, err := getAPIKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get API key: %w", err)
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}

	return &AIRouter{
		apiKey: apiKey,
		client: client,
	}, nil
}

// getAPIKey はSecret ManagerからGemini APIキーを取得
func getAPIKey(ctx context.Context) (string, error) {
	// Secret Managerクライアントを作成
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		// フォールバック: 環境変数から取得
		log.Printf("Secret Manager client creation failed: %v, falling back to env var", err)
		return getFallbackAPIKey()
	}
	defer client.Close()

	// Secret Managerからキーを取得
	name := fmt.Sprintf("projects/%s/secrets/%s/versions/latest",
		config.GCPProjectID,
		config.SecretGeminiAPIKey)

	req := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}

	result, err := client.AccessSecretVersion(ctx, req)
	if err != nil {
		// フォールバック: 環境変数から取得
		log.Printf("Secret Manager access failed: %v, falling back to env var", err)
		return getFallbackAPIKey()
	}

	return string(result.Payload.Data), nil
}

// getFallbackAPIKey は環境変数からAPIキーを取得
func getFallbackAPIKey() (string, error) {
	apiKey := config.GetEnv("GEMINI_API_KEY", "")
	if apiKey == "" {
		return "", fmt.Errorf("GEMINI_API_KEY not found in environment variables")
	}
	return apiKey, nil
}

// AnalyzeDocument はドキュメントを解析（AIルーターパターン）
func (r *AIRouter) AnalyzeDocument(ctx context.Context, data []byte, mimeType string, prompt string, useFlashFirst bool) (*model.AnalysisResult, error) {
	if useFlashFirst {
		// 第一段階: Gemini Flash
		log.Println("Gemini Flash で解析開始")
		flashResult, err := r.callGemini(ctx, config.GeminiModelsConfig.Flash, data, mimeType, prompt)
		if err == nil && r.isConfident(flashResult) {
			log.Printf("Flash解析成功（信頼度: %.2f）", flashResult.ConfidenceScore)
			return flashResult, nil
		}

		// 第二段階: Gemini Proへエスカレーション
		if config.AIRouter.EnableProEscalation {
			score := 0.0
			if flashResult != nil {
				score = flashResult.ConfidenceScore
			}
			log.Printf("信頼度が低いためProにエスカレーション (score: %.2f)", score)
			return r.callGemini(ctx, config.GeminiModelsConfig.Pro, data, mimeType, prompt)
		}

		return flashResult, err
	}

	// Pro直接呼び出し
	return r.callGemini(ctx, config.GeminiModelsConfig.Pro, data, mimeType, prompt)
}

// callGemini はGemini APIを呼び出し
func (r *AIRouter) callGemini(ctx context.Context, modelName string, data []byte, mimeType string, prompt string) (*model.AnalysisResult, error) {
	genModel := r.client.GenerativeModel(modelName) // Rename variable to avoid shadowing package name
	genModel.GenerationConfig.ResponseMIMEType = "application/json"

	// 画像/PDFとプロンプトを送信
	// mimeTypeがimage/jpeg等の場合はImageDataでも良いが、汎用的にBlobを使用可能か確認
	// genai.ImageDataは shortcut for Blob(mimeType, data) with validation for image types?
	// PDFの場合はBlobを使う必要がある。

	var dataPart genai.Part
	if mimeType == "application/pdf" {
		dataPart = genai.Blob{
			MIMEType: mimeType,
			Data:     data,
		}
	} else {
		// 画像（JPEG, PNG等）
		// mimeTypeから拡張子部分を取得（簡易的）
		format := "jpeg"
		if len(mimeType) > 6 {
			format = mimeType[6:]
		}
		dataPart = genai.ImageData(format, data)
	}

	resp, err := genModel.GenerateContent(ctx,
		dataPart,
		genai.Text(prompt),
	)
	if err != nil {
		return nil, fmt.Errorf("gemini API call failed (%s): %w", modelName, err)
	}

	// JSONレスポンスをパース
	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from gemini API")
	}

	// レスポンステキストを取得
	text := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	var result model.AnalysisResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &result, nil
}

// isConfident は信頼度スコアをチェック
func (r *AIRouter) isConfident(result *model.AnalysisResult) bool {
	if result == nil {
		return false
	}
	return result.ConfidenceScore >= config.AIRouter.ConfidenceThreshold
}

// ExtractEventsAndTasks はドキュメントから予定とタスクを抽出
func (r *AIRouter) ExtractEventsAndTasks(ctx context.Context, data []byte, mimeType string, fileName string) (*model.EventsAndTasks, error) {
	today := time.Now().Format("2006-01-02") // Use time.Now()

	prompt := fmt.Sprintf(`
あなたは学校のお便りから予定とタスクを抽出するアシスタントです。
以下の画像を解析し、JSON形式で回答してください。

## 出力形式（必ずこのJSON形式で回答）
{
  "events": [
    {
      "title": "イベントタイトル",
      "date": "YYYY-MM-DD",
      "start_time": "HH:MM（不明な場合は null）",
      "end_time": "HH:MM（不明な場合は null）",
      "location": "場所（不明な場合は null）",
      "description": "詳細説明"
    }
  ],
  "tasks": [
    {
      "title": "タスクタイトル（例：○○の提出）",
      "due_date": "YYYY-MM-DD",
      "notes": "備考"
    }
  ]
}

## 判断基準
- **events**: 日時が確定している行事（運動会、授業参観、保護者会など）
- **tasks**: 期限がある提出物や準備事項（書類提出、持ち物準備など）

## 注意事項
- 過去の日付（%sより前）のイベント・タスクは除外してください
- 年が明示されていない場合は、%s年と仮定してください
- 抽出できる情報がない場合は、eventsとtasksを空配列にしてください

## ファイル名
%s
`, today, today[:4], fileName)

	genModel := r.client.GenerativeModel(config.GeminiModelsConfig.Flash) // Rename to genModel
	genModel.GenerationConfig.ResponseMIMEType = "application/json"

	var dataPart genai.Part
	if mimeType == "application/pdf" {
		dataPart = genai.Blob{
			MIMEType: mimeType,
			Data:     data,
		}
	} else {
		format := "jpeg"
		if len(mimeType) > 6 {
			format = mimeType[6:]
		}
		dataPart = genai.ImageData(format, data)
	}

	resp, err := genModel.GenerateContent(ctx,
		dataPart,
		genai.Text(prompt),
	)
	if err != nil {
		return nil, fmt.Errorf("gemini API call failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no response from gemini API")
	}

	text := fmt.Sprintf("%v", resp.Candidates[0].Content.Parts[0])

	var result model.EventsAndTasks
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &result, nil
}

// ExtractOCRText はドキュメントからプレーンテキストを抽出（OCR）- 互換性維持用ラッパー
func (r *AIRouter) ExtractOCRText(ctx context.Context, data []byte, mimeType string) (string, error) {
	bundle, err := r.ExtractOCRBundle(ctx, data, mimeType)
	if err != nil {
		return "", err
	}
	return bundle.OCRText, nil
}

// ExtractOCRBundle はドキュメントからOCRテキスト、事実、要約を構造化して抽出
func (r *AIRouter) ExtractOCRBundle(ctx context.Context, data []byte, mimeType string) (*model.OCRBundle, error) {
	prompt := `この画像/ドキュメントを解析し、以下のJSON形式で回答してください。

{
  "ocr_text": "画像/ドキュメントに含まれるすべてのテキスト（OCR結果）",
  "facts": [
    "実際に本文から抽出した項目のみ"
  ],
  "summary": "要約（1-2文、曖昧な場合は空文字）",
  "confidence_score": 0.0-1.0,
  "quality": {
    "uncertain": false,
    "needs_high_model": false,
    "notes": "読み取りに問題があれば記載"
  }
}

重要ルール：
- factsには実際に本文から抽出した項目のみを入れてください。説明文やテンプレ文は絶対に含めないでください。
- factsには日付、金額、名前、住所など具体的な情報を抽出。推測は禁止。
- 見つからない場合は "不明" を入れてください。最大10項目。
- 読み取れない部分がある場合は quality.uncertain を true に。
- summaryは確信がなければ空文字で可。`

	genModel := r.client.GenerativeModel(config.GeminiModelsConfig.Flash)
	genModel.GenerationConfig.ResponseMIMEType = "application/json"

	var dataPart genai.Part
	if mimeType == "application/pdf" {
		dataPart = genai.Blob{
			MIMEType: mimeType,
			Data:     data,
		}
	} else {
		format := "jpeg"
		if len(mimeType) > 6 {
			format = mimeType[6:]
		}
		dataPart = genai.ImageData(format, data)
	}

	resp, err := genModel.GenerateContent(ctx,
		dataPart,
		genai.Text(prompt),
	)
	if err != nil {
		return nil, fmt.Errorf("OCR extraction failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return &model.OCRBundle{}, nil
	}

	// Partsを連結してテキストを取得
	text := r.extractTextFromParts(resp.Candidates[0].Content.Parts)

	var bundle model.OCRBundle
	if err := json.Unmarshal([]byte(text), &bundle); err != nil {
		// JSONパース失敗時はプレーンテキストとして扱う
		log.Printf("OCRBundle JSONパース失敗、プレーンテキストとして処理: %v", err)
		bundle.OCRText = text
		bundle.ConfidenceScore = 0.5
	}

	// Flash結果が弱い場合はProにエスカレーション
	if bundle.ConfidenceScore < config.AIRouter.ConfidenceThreshold ||
		bundle.Quality.NeedsHighModel ||
		(bundle.Quality.Uncertain && len(bundle.Facts) < 3) ||
		len(bundle.OCRText) < 200 {
		if config.AIRouter.EnableProEscalation {
			log.Printf("OCR精度が低いためProにエスカレーション (score: %.2f, len: %d)", bundle.ConfidenceScore, len(bundle.OCRText))
			return r.extractOCRBundleWithModel(ctx, data, mimeType, config.GeminiModelsConfig.Pro, prompt)
		}
	}

	log.Printf("OCRBundle抽出完了: %d文字, %d facts", len(bundle.OCRText), len(bundle.Facts))
	return &bundle, nil
}

// extractTextFromParts はPartsからテキストを安全に抽出
func (r *AIRouter) extractTextFromParts(parts []genai.Part) string {
	var result string
	for _, part := range parts {
		switch v := part.(type) {
		case genai.Text:
			result += string(v)
		default:
			// Text以外のPartは文字列化
			result += fmt.Sprintf("%v", v)
		}
	}
	return result
}

// extractOCRBundleWithModel は指定モデルでOCRBundle抽出
func (r *AIRouter) extractOCRBundleWithModel(ctx context.Context, data []byte, mimeType, modelName, prompt string) (*model.OCRBundle, error) {
	genModel := r.client.GenerativeModel(modelName)
	genModel.GenerationConfig.ResponseMIMEType = "application/json"

	var dataPart genai.Part
	if mimeType == "application/pdf" {
		dataPart = genai.Blob{
			MIMEType: mimeType,
			Data:     data,
		}
	} else {
		format := "jpeg"
		if len(mimeType) > 6 {
			format = mimeType[6:]
		}
		dataPart = genai.ImageData(format, data)
	}

	resp, err := genModel.GenerateContent(ctx,
		dataPart,
		genai.Text(prompt),
	)
	if err != nil {
		return nil, fmt.Errorf("OCR extraction (Pro) failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return &model.OCRBundle{}, nil
	}

	// Partsを連結してテキストを取得
	text := r.extractTextFromParts(resp.Candidates[0].Content.Parts)

	var bundle model.OCRBundle
	if err := json.Unmarshal([]byte(text), &bundle); err != nil {
		bundle.OCRText = text
		bundle.ConfidenceScore = 0.5
	}

	log.Printf("OCRBundle抽出完了 (Pro): %d文字, %d facts", len(bundle.OCRText), len(bundle.Facts))
	return &bundle, nil
}

// Close はクライアントをクローズ
func (r *AIRouter) Close() error {
	return r.client.Close()
}
