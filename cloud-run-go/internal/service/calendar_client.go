package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/leo-sagawa/homedocmanager/internal/config"
	"github.com/leo-sagawa/homedocmanager/internal/model"
)

// CalendarClient はGoogle Calendar APIクライアント
type CalendarClient struct {
	oauthCreds *OAuthCredentials
	calendarID string
}

// NewCalendarClient は新しいCalendarClientを作成
func NewCalendarClient(ctx context.Context) (*CalendarClient, error) {
	creds, err := GetOAuthCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth credentials for Calendar: %w", err)
	}

	return &CalendarClient{
		oauthCreds: creds,
		calendarID: config.CalendarID,
	}, nil
}

// CreateEvent はカレンダーイベントを作成
func (cc *CalendarClient) CreateEvent(ctx context.Context, event *model.Event, notes string) (string, error) {
	accessToken, err := cc.oauthCreds.GetAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// イベント本文を構築
	description := event.Description
	if notes != "" {
		if description != "" {
			description += "\n\n"
		}
		description += notes
	}

	eventBody := map[string]interface{}{
		"summary":     event.Title,
		"description": description,
	}

	if event.Location != nil && *event.Location != "" {
		eventBody["location"] = *event.Location
	}

	// 日時の設定
	if event.StartTime != nil && *event.StartTime != "" {
		// 時間指定イベント
		startDT, err := parseDateTime(event.Date, *event.StartTime)
		if err != nil {
			return "", fmt.Errorf("failed to parse start time: %w", err)
		}

		endDT := startDT.Add(time.Hour) // デフォルト1時間
		if event.EndTime != nil && *event.EndTime != "" {
			endDT, err = parseDateTime(event.Date, *event.EndTime)
			if err != nil {
				return "", fmt.Errorf("failed to parse end time: %w", err)
			}
		}

		eventBody["start"] = map[string]string{
			"dateTime": startDT.Format(time.RFC3339),
			"timeZone": "Asia/Tokyo",
		}
		eventBody["end"] = map[string]string{
			"dateTime": endDT.Format(time.RFC3339),
			"timeZone": "Asia/Tokyo",
		}
		log.Printf("時間指定イベント作成: %s (%s)", event.Title, startDT.Format("2006-01-02 15:04"))
	} else {
		// 終日イベント
		startDate, err := parseDate(event.Date)
		if err != nil {
			return "", fmt.Errorf("failed to parse date: %w", err)
		}
		endDate := startDate.AddDate(0, 0, 1) // 終日イベントは翌日まで（exclusive）

		eventBody["start"] = map[string]string{
			"date": startDate.Format("2006-01-02"),
		}
		eventBody["end"] = map[string]string{
			"date": endDate.Format("2006-01-02"),
		}
		log.Printf("終日イベント作成: %s (%s)", event.Title, event.Date)
	}

	// APIリクエスト
	jsonBody, err := json.Marshal(eventBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal event: %w", err)
	}

	url := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events", cc.calendarID)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create event: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("calendar API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		HTMLLink string `json:"htmlLink"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("イベント作成成功: %s", event.Title)
	return result.HTMLLink, nil
}

// EventExists は同じタイトルと日付のイベントが既に存在するかチェック
func (cc *CalendarClient) EventExists(ctx context.Context, title string, dateStr string) (bool, error) {
	accessToken, err := cc.oauthCreds.GetAccessToken(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get access token: %w", err)
	}

	startDate, err := parseDate(dateStr)
	if err != nil {
		return false, err
	}
	endDate := startDate.AddDate(0, 0, 1)

	apiURL := fmt.Sprintf("https://www.googleapis.com/calendar/v3/calendars/%s/events?timeMin=%s&timeMax=%s&q=%s",
		cc.calendarID,
		url.QueryEscape(startDate.Format(time.RFC3339)),
		url.QueryEscape(endDate.Format(time.RFC3339)),
		url.QueryEscape(title),
	)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, fmt.Errorf("calendar API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Items []struct {
			Summary string `json:"summary"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	// タイトルが完全一致するものを探す（qパラメータは部分一致のため）
	for _, item := range result.Items {
		if item.Summary == title {
			return true, nil
		}
	}

	return false, nil
}

// parseDateTime は日付と時刻文字列をパース
func parseDateTime(dateStr string, timeStr string) (time.Time, error) {
	// 日付フォーマット: "2006-01-02" または "20060102"
	// 時刻フォーマット: "15:04"
	var date time.Time
	var err error

	if len(dateStr) == 8 {
		date, err = time.ParseInLocation("20060102", dateStr, time.FixedZone("Asia/Tokyo", 9*60*60))
	} else {
		date, err = time.ParseInLocation("2006-01-02", dateStr, time.FixedZone("Asia/Tokyo", 9*60*60))
	}
	if err != nil {
		return time.Time{}, err
	}

	// 時刻をパース
	timeParts := timeStr
	var hour, min int
	fmt.Sscanf(timeParts, "%d:%d", &hour, &min)

	return time.Date(date.Year(), date.Month(), date.Day(), hour, min, 0, 0, time.FixedZone("Asia/Tokyo", 9*60*60)), nil
}

// parseDate は日付文字列をパース
func parseDate(dateStr string) (time.Time, error) {
	if len(dateStr) == 8 {
		return time.ParseInLocation("20060102", dateStr, time.FixedZone("Asia/Tokyo", 9*60*60))
	}
	return time.ParseInLocation("2006-01-02", dateStr, time.FixedZone("Asia/Tokyo", 9*60*60))
}
