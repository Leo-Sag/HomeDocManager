package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/leo-sagawa/homedocmanager/internal/model"
)

// TasksClient はGoogle Tasks APIクライアント
type TasksClient struct {
	oauthCreds *OAuthCredentials
}

// NewTasksClient は新しいTasksClientを作成
func NewTasksClient(ctx context.Context) (*TasksClient, error) {
	creds, err := GetOAuthCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth credentials for Tasks: %w", err)
	}

	return &TasksClient{
		oauthCreds: creds,
	}, nil
}

// CreateTask はタスクを作成
func (tc *TasksClient) CreateTask(ctx context.Context, task *model.Task, notes string) (string, error) {
	accessToken, err := tc.oauthCreds.GetAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// タスク本文を構築
	taskNotes := task.Notes
	if notes != "" {
		if taskNotes != "" {
			taskNotes += "\n\n"
		}
		taskNotes += notes
	}

	taskBody := map[string]interface{}{
		"title": task.Title,
		"notes": taskNotes,
	}

	// 期日の設定
	if task.DueDate != "" {
		// Tasks APIはRFC3339形式の日時を要求
		var dueDate string
		if len(task.DueDate) == 8 {
			// YYYYMMDD形式
			dueDate = fmt.Sprintf("%s-%s-%sT00:00:00Z", task.DueDate[:4], task.DueDate[4:6], task.DueDate[6:8])
		} else if len(task.DueDate) == 10 {
			// YYYY-MM-DD形式
			dueDate = task.DueDate + "T00:00:00Z"
		}
		if dueDate != "" {
			taskBody["due"] = dueDate
		}
	}

	// APIリクエスト
	jsonBody, err := json.Marshal(taskBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal task: %w", err)
	}

	url := "https://tasks.googleapis.com/tasks/v1/lists/@default/tasks"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tasks API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	log.Printf("タスク作成成功: %s", task.Title)
	return result.ID, nil
}

// TaskExists は同じタイトルの未完了タスクが既に存在するかチェック
func (tc *TasksClient) TaskExists(ctx context.Context, title string) (bool, error) {
	accessToken, err := tc.oauthCreds.GetAccessToken(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get access token: %w", err)
	}

	// 未完了（needsAction）タスクのみ取得して確認
	url := "https://tasks.googleapis.com/tasks/v1/lists/@default/tasks?showCompleted=false"
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
		return false, fmt.Errorf("tasks API error: %s - %s", resp.Status, string(body))
	}

	var result struct {
		Items []struct {
			Title string `json:"title"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, err
	}

	for _, item := range result.Items {
		if item.Title == title {
			return true, nil
		}
	}

	return false, nil
}
