package linebot

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

type Handler struct {
	bot     *linebot.Client
	service *Service
}

// NewHandler は新しいLINE Webhookハンドラーを作成
func NewHandler(channelSecret, accessToken string, service *Service) (*Handler, error) {
	bot, err := linebot.New(channelSecret, accessToken)
	if err != nil {
		return nil, err
	}
	return &Handler{
		bot:     bot,
		service: service,
	}, nil
}

// HandleWebhook はLINEからのWebhookイベントを処理
func (h *Handler) HandleWebhook(c *gin.Context) {
	events, err := h.bot.ParseRequest(c.Request)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
		}
		return
	}

	for _, event := range events {
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				h.handleTextMessage(event.ReplyToken, message.Text)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

func (h *Handler) handleTextMessage(replyToken, text string) {
	// Flex Messageを生成
	category, flexContents, err := h.service.BuildFlexMessage(text)
	if err != nil {
		log.Printf("Error building flex message: %v", err)
		return
	}

	// map[string]interface{} から linebot.FlexContainer へ変換
	b, err := json.Marshal(flexContents)
	if err != nil {
		log.Printf("Error marshaling flex message: %v", err)
		return
	}
	container, err := linebot.UnmarshalFlexMessageJSON(b)
	if err != nil {
		log.Printf("Error unmarshaling flex message: %v", err)
		return
	}

	// Quick Replyを追加
	quickReplyItems := h.service.GetQuickReplyItems(category)
	var qrItems []*linebot.QuickReplyButton
	for _, item := range quickReplyItems {
		// 安全に取り出す（型崩れ・設定ミスでもpanicしない）
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

		qrItems = append(qrItems, linebot.NewQuickReplyButton(
			"", // 画像なし
			linebot.NewMessageAction(label, text),
		))
	}

	msg := linebot.NewFlexMessage("NotebookLM案内", container)
	if len(qrItems) > 0 {
		msg.WithQuickReplies(linebot.NewQuickReplyItems(qrItems...))
	}

	res := h.bot.ReplyMessage(replyToken, msg)

	if _, err := res.Do(); err != nil {
		log.Printf("Error replying message: %v", err)
	}
}
