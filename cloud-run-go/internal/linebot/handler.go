package linebot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/line/line-bot-sdk-go/v7/linebot"
)

type Handler struct {
	bot        *linebot.Client
	service    *Service
	ragService *RAGService
}

// NewHandler は新しいLINE Webhookハンドラーを作成
func NewHandler(channelSecret, accessToken string, service *Service, ragService *RAGService) (*Handler, error) {
	bot, err := linebot.New(channelSecret, accessToken)
	if err != nil {
		return nil, err
	}
	return &Handler{
		bot:        bot,
		service:    service,
		ragService: ragService,
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
				userID := ""
				groupID := ""
				sourceType := "unknown"
				if event.Source != nil {
					userID = event.Source.UserID
					groupID = event.Source.GroupID
					sourceType = string(event.Source.Type)
				}
				// UserIDをログに出力（設定用）
				log.Printf("[LINE] Message received - UserID: %s, GroupID: %s, SourceType: %s, Text: %s",
					userID, groupID, sourceType, truncateText(message.Text, 50))
				h.handleTextMessage(event.ReplyToken, userID, groupID, message.Text)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "OK"})
}

// truncateText はテキストを指定長で切り詰める
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}

func (h *Handler) handleTextMessage(replyToken, userID, groupID, text string) {
	// 家族グループからのメッセージの場合、未知のユーザーを自動識別
	if groupID != "" && h.ragService != nil && !h.ragService.IsUserKnown(userID) {
		h.autoIdentifyUser(userID, groupID)
	}

	// 管理コマンド: #メンバー確認 / #myid
	if text == "#メンバー確認" || text == "#myid" {
		h.handleMyIDCommand(replyToken, userID, groupID)
		return
	}

	// 管理コマンド: #メンバー登録 (グループメンバーを走査して紐付け)
	if text == "#メンバー登録" && groupID != "" {
		h.handleSyncMembersCommand(replyToken, groupID)
		return
	}

	// 管理コマンド: #RAG更新 / #rag (フォルダ内ドキュメントの再スキャン)
	if (text == "#RAG更新" || text == "#rag") && h.ragService != nil {
		h.handleRefreshRAGCommand(replyToken)
		return
	}

	// トリガーワードでなければRAGモードで処理
	if h.ragService != nil && !h.service.IsTriggerWord(text) {
		// カテゴリプレフィックスのみの場合はヘルプメッセージを返す
		if helpMsg := h.getCategoryHelpMessage(text); helpMsg != "" {
			if _, err := h.bot.ReplyMessage(replyToken, linebot.NewTextMessage(helpMsg)).Do(); err != nil {
				log.Printf("Error replying category help: %v", err)
			}
			return
		}
		h.handleRAGQuery(replyToken, userID, text)
		return
	}

	// Flex Messageを生成（既存ロジック）
	category, flexContents, err := h.service.BuildFlexMessage(text)
	if err != nil {
		log.Printf("Error building flex message: %v", err)
		return
	}

	// altText と payload を正規化
	altText := "NotebookLM案内"
	payload := flexContents

	// テンプレに altText があれば採用
	if v, ok := flexContents["altText"].(string); ok && v != "" {
		altText = v
	}

	// contents ラッパーがある場合は bubble 本体だけ抜く
	if cAny, ok := flexContents["contents"]; ok {
		if cMap, ok := cAny.(map[string]interface{}); ok {
			payload = cMap
		}
	}

	// bubble本体だけを Unmarshal する
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

	// altText を反映して返信する
	msg := linebot.NewFlexMessage(altText, container)

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

	if len(qrItems) > 0 {
		msg.WithQuickReplies(linebot.NewQuickReplyItems(qrItems...))
	}

	res := h.bot.ReplyMessage(replyToken, msg)

	if _, err := res.Do(); err != nil {
		log.Printf("Error replying message: %v", err)
	}
}

// handleRAGQuery はRAGクエリを処理して回答を返信
func (h *Handler) handleRAGQuery(replyToken, userID, query string) {
	ctx := context.Background()
	response, err := h.ragService.GenerateAnswer(ctx, userID, query)
	if err != nil {
		log.Printf("RAG query error for user %s: %v", userID, err)
		h.replyErrorMessage(replyToken, "申し訳ございません。処理中にエラーが発生しました。しばらくしてからもう一度お試しください。")
		return
	}

	if _, err := h.bot.ReplyMessage(replyToken, linebot.NewTextMessage(response)).Do(); err != nil {
		log.Printf("Error replying RAG response: %v", err)
	}
}

// replyErrorMessage はエラーメッセージを返信
func (h *Handler) replyErrorMessage(replyToken, message string) {
	if _, err := h.bot.ReplyMessage(replyToken, linebot.NewTextMessage(message)).Do(); err != nil {
		log.Printf("Error replying error message: %v", err)
	}
}

// getCategoryHelpMessage はカテゴリプレフィックスのみの入力を検出してヘルプメッセージを返す
// 具体的な質問がある場合は空文字を返す
func (h *Handler) getCategoryHelpMessage(text string) string {
	// カテゴリプレフィックスと対応するヘルプメッセージ
	categoryHelp := map[string]string{
		"生活：":    "🏠 生活についてですね！\n\n例えば以下のように続けて質問してください：\n• 「生活：火災保険の更新はいつ？」\n• 「生活：自治体の封筒の内容は？」\n\n💡 質問を入力してこのメッセージに返信してください。",
		"お金：":    "💰 お金についてですね！\n\n例えば以下のように続けて質問してください：\n• 「お金：生命保険の保障内容は？」\n• 「お金：ふるさと納税先は？」\n\n💡 質問を入力してこのメッセージに返信してください。",
		"子供：":    "👶 子供についてですね！\n\n例えば以下のように続けて質問してください：\n• 「子供：提出物の締切は？」\n• 「子供：習い事の連絡先は？」\n\n💡 質問を入力してこのメッセージに返信してください。",
		"医療：":    "🏥 医療についてですね！\n\n例えば以下のように続けて質問してください：\n• 「医療：予防接種の予定は？」\n• 「医療：診療明細の内容は？」\n\n💡 質問を入力してこのメッセージに返信してください。",
		"ライブラリ：": "📚 ライブラリについてですね！\n\n例えば以下のように続けて質問してください：\n• 「ライブラリ：家電のエラー対処法は？」\n• 「ライブラリ：取説PDFはどこ？」\n\n💡 質問を入力してこのメッセージに返信してください。",
	}

	// 入力がカテゴリプレフィックスのみかチェック
	trimmedText := strings.TrimSpace(text)
	if helpMsg, exists := categoryHelp[trimmedText]; exists {
		return helpMsg
	}

	return ""
}

// handleMyIDCommand はユーザーIDを返信する管理コマンド
func (h *Handler) handleMyIDCommand(replyToken, userID, groupID string) {
	msg := "📋 あなたのLINE情報\n\n"
	msg += "🆔 User ID:\n" + userID + "\n"
	if groupID != "" {
		msg += "\n👥 Group ID:\n" + groupID
	}
	msg += "\n\n💡 このUser IDをline_user_settings.jsonに登録してください。"

	log.Printf("[LINE] MyID command - UserID: %s, GroupID: %s", userID, groupID)

	if _, err := h.bot.ReplyMessage(replyToken, linebot.NewTextMessage(msg)).Do(); err != nil {
		log.Printf("Error replying myid: %v", err)
	}
}

// GetGroupMemberIDs はグループのメンバーIDを取得（管理エンドポイント用）
func (h *Handler) GetGroupMemberIDs(groupID string) ([]string, error) {
	var memberIDs []string
	nextToken := ""

	for {
		resp, err := h.bot.GetGroupMemberIDs(groupID, nextToken).Do()
		if err != nil {
			return nil, err
		}
		memberIDs = append(memberIDs, resp.MemberIDs...)
		if resp.Next == "" {
			break
		}
		nextToken = resp.Next
	}

	return memberIDs, nil
}

// GetGroupMemberProfile はグループ内のメンバープロフィールを取得
func (h *Handler) GetGroupMemberProfile(groupID, userID string) (*linebot.UserProfileResponse, error) {
	return h.bot.GetGroupMemberProfile(groupID, userID).Do()
}

// autoIdentifyUser は特定のユーザーの識別を試行
func (h *Handler) autoIdentifyUser(userID, groupID string) {
	profile, err := h.GetGroupMemberProfile(groupID, userID)
	if err != nil {
		log.Printf("[LINE] Failed to get profile for auto-identify: %v", err)
		return
	}

	name := h.ragService.IdentifyUserByDisplayName(profile.DisplayName)
	if name != "" {
		h.ragService.UpdateUser(userID, name)
		log.Printf("[LINE] Auto-identified user: %s as %s", userID, name)
	}
}

// handleSyncMembersCommand はグループメンバー全員を走査して識別
func (h *Handler) handleSyncMembersCommand(replyToken, groupID string) {
	memberIDs, err := h.GetGroupMemberIDs(groupID)
	if err != nil {
		log.Printf("[LINE] Failed to get member IDs: %v", err)
		h.replyErrorMessage(replyToken, "メンバー一覧の取得に失敗しました。")
		return
	}

	identified := 0
	msg := "📄 メンバー登録状況:\n"
	for _, id := range memberIDs {
		profile, err := h.GetGroupMemberProfile(groupID, id)
		if err != nil {
			continue
		}
		name := h.ragService.IdentifyUserByDisplayName(profile.DisplayName)
		if name != "" {
			h.ragService.UpdateUser(id, name)
			msg += fmt.Sprintf("✅ %s -> %s\n", profile.DisplayName, name)
			log.Printf("[LINE] Identified member: %s (%s) as %s", profile.DisplayName, id, name)
			identified++
		} else {
			msg += fmt.Sprintf("❓ %s (未登録)\n", profile.DisplayName)
			log.Printf("[LINE] Unidentified member: %s (%s)", profile.DisplayName, id)
		}
	}

	msg += fmt.Sprintf("\n合計 %d 名の大人メンバーを識別しました。", identified)
	h.bot.ReplyMessage(replyToken, linebot.NewTextMessage(msg)).Do()
}

// handleRefreshRAGCommand はRAGキャッシュを強制更新する
func (h *Handler) handleRefreshRAGCommand(replyToken string) {
	ctx := context.Background()
	_, err := h.ragService.RefreshCache(ctx)
	if err != nil {
		log.Printf("[RAG] Manual refresh failed: %v", err)
		h.replyErrorMessage(replyToken, "RAG知識の更新に失敗しました。詳細なエラー内容はログを確認してください。")
		return
	}

	msg := "✅ RAG知識を更新しました。\n対象フォルダ内のGoogleドキュメントを再読み込みしました。"
	if _, err := h.bot.ReplyMessage(replyToken, linebot.NewTextMessage(msg)).Do(); err != nil {
		log.Printf("Error replying rag refresh: %v", err)
	}
}
