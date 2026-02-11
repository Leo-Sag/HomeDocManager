package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// DiscordNotifier はDiscord Webhookで通知を送信するクライアント
type DiscordNotifier struct {
	webhookURL string
	httpClient *http.Client
}

// NewDiscordNotifier は新しいDiscordNotifierを作成する。URLが空の場合はnilを返す。
func NewDiscordNotifier(webhookURL string) *DiscordNotifier {
	if webhookURL == "" {
		return nil
	}
	return &DiscordNotifier{
		webhookURL: webhookURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// FileDetail はInboxスキャン結果の各ファイル情報
type FileDetail struct {
	Name   string
	Result string
}

// discordEmbed はDiscord Embed構造体
type discordEmbed struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Color       int            `json:"color"`
	Fields      []discordField `json:"fields,omitempty"`
	Timestamp   string         `json:"timestamp"`
}

type discordField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordPayload struct {
	Embeds []discordEmbed `json:"embeds"`
}

const (
	colorRed    = 0xFF0000
	colorYellow = 0xFFFF00
	colorGreen  = 0x00CC00
)

// NotifyError はファイル処理エラーをDiscordに通知する
func (d *DiscordNotifier) NotifyError(fileName, errorMsg string) {
	if d == nil {
		return
	}

	embed := discordEmbed{
		Title: "ファイル処理エラー",
		Color: colorRed,
		Fields: []discordField{
			{Name: "ファイル", Value: fileName, Inline: true},
			{Name: "エラー", Value: truncate(errorMsg, 1024), Inline: false},
		},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	d.send(discordPayload{Embeds: []discordEmbed{embed}})
}

// NotifyInboxScanResult はInboxスキャン結果をDiscordに通知する
// ファイル0件または全件SKIPPEDの場合は通知しない
func (d *DiscordNotifier) NotifyInboxScanResult(total, processed, skipped, errors int, details []FileDetail) {
	if d == nil {
		return
	}

	// 通知不要: ファイルなし、または全件スキップ（既に処理済み）
	if total == 0 || (processed == 0 && errors == 0) {
		return
	}

	color := colorGreen
	if errors > 0 {
		color = colorYellow
	}

	// ファイル一覧を構築
	var lines []string
	for _, detail := range details {
		icon := "✅"
		switch detail.Result {
		case "SKIPPED":
			icon = "⏭️"
		case "ERROR":
			icon = "❌"
		}
		lines = append(lines, fmt.Sprintf("%s %s", icon, detail.Name))
	}
	fileList := strings.Join(lines, "\n")
	if len(fileList) > 4000 {
		fileList = fileList[:4000] + "\n..."
	}

	embed := discordEmbed{
		Title: "Inbox スキャン完了",
		Description: fmt.Sprintf("処理: **%d件** | スキップ: **%d件** | エラー: **%d件**\n\n%s",
			processed, skipped, errors, fileList),
		Color:     color,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	d.send(discordPayload{Embeds: []discordEmbed{embed}})
}

// send はDiscord Webhookにペイロードを送信する
func (d *DiscordNotifier) send(payload discordPayload) {
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Discord通知: JSONエンコード失敗: %v", err)
		return
	}

	resp, err := d.httpClient.Post(d.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Discord通知: 送信失敗: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("Discord通知: HTTPエラー: %d", resp.StatusCode)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
