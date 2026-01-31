package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// PhotosClient はGoogle Photos APIクライアント
type PhotosClient struct {
	oauthCreds *OAuthCredentials
}

// NewPhotosClient は新しいPhotosClientを作成
func NewPhotosClient(ctx context.Context) (*PhotosClient, error) {
	creds, err := GetOAuthCredentials(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth credentials for Photos: %w", err)
	}

	return &PhotosClient{
		oauthCreds: creds,
	}, nil
}

// UploadImage は画像をGoogle Photosにアップロード（2段階プロトコル）
func (pc *PhotosClient) UploadImage(ctx context.Context, imageData []byte, description string) (string, error) {
	accessToken, err := pc.oauthCreds.GetAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get access token: %w", err)
	}

	// 第一段階: バイトアップロード
	uploadToken, err := pc.uploadBytes(ctx, accessToken, imageData)
	if err != nil {
		return "", fmt.Errorf("byte upload failed: %w", err)
	}

	// 第二段階: メディアアイテム作成
	productURL, err := pc.createMediaItem(ctx, accessToken, uploadToken, description)
	if err != nil {
		return "", fmt.Errorf("media item creation failed: %w", err)
	}

	return productURL, nil
}

// uploadBytes は第一段階のバイトアップロード
func (pc *PhotosClient) uploadBytes(ctx context.Context, accessToken string, imageData []byte) (string, error) {
	url := "https://photoslibrary.googleapis.com/v1/uploads"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(imageData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Goog-Upload-Content-Type", "image/jpeg")
	req.Header.Set("X-Goog-Upload-Protocol", "raw")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("upload failed: %s - %s", resp.Status, string(body))
	}

	log.Println("バイトアップロード成功")
	return string(body), nil // Upload Token
}

// createMediaItem は第二段階のメディアアイテム作成
func (pc *PhotosClient) createMediaItem(ctx context.Context, accessToken, uploadToken, description string) (string, error) {
	url := "https://photoslibrary.googleapis.com/v1/mediaItems:batchCreate"

	payload := map[string]interface{}{
		"newMediaItems": []map[string]interface{}{
			{
				"description": description,
				"simpleMediaItem": map[string]string{
					"uploadToken": uploadToken,
				},
			},
		},
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create media item request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create media item failed: %s - %s", resp.Status, string(body))
	}

	var result struct {
		NewMediaItemResults []struct {
			MediaItem struct {
				ID         string `json:"id"`
				ProductURL string `json:"productUrl"`
			} `json:"mediaItem"`
		} `json:"newMediaItemResults"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if len(result.NewMediaItemResults) == 0 || result.NewMediaItemResults[0].MediaItem.ID == "" {
		return "", fmt.Errorf("no media item created")
	}

	mediaItem := result.NewMediaItemResults[0].MediaItem
	log.Printf("メディアアイテム作成成功: %s", mediaItem.ID)
	return mediaItem.ProductURL, nil
}
