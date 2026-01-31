package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/leo-sagawa/homedocmanager/internal/config"
)

// OAuthCredentials はOAuth認証情報を保持
type OAuthCredentials struct {
	AccessToken  string
	RefreshToken string
	ClientID     string
	ClientSecret string
}

var (
	oauthCreds     *OAuthCredentials
	oauthCredsOnce sync.Once
	oauthCredsMu   sync.Mutex
)

// GetOAuthCredentials はOAuth認証情報を取得（シングルトン）
func GetOAuthCredentials(ctx context.Context) (*OAuthCredentials, error) {
	var initErr error
	oauthCredsOnce.Do(func() {
		creds, err := loadOAuthCredentials(ctx)
		if err != nil {
			initErr = err
			return
		}
		oauthCreds = creds
	})

	if initErr != nil {
		return nil, initErr
	}
	return oauthCreds, nil
}

// GetAccessToken は有効なアクセストークンを取得（必要に応じてリフレッシュ）
func (c *OAuthCredentials) GetAccessToken(ctx context.Context) (string, error) {
	oauthCredsMu.Lock()
	defer oauthCredsMu.Unlock()

	// TODO: トークン有効期限チェック（現状は毎回リフレッシュ）
	if c.AccessToken == "" {
		if err := c.refreshToken(ctx); err != nil {
			return "", err
		}
	}

	return c.AccessToken, nil
}

// refreshToken はアクセストークンをリフレッシュ
func (c *OAuthCredentials) refreshToken(ctx context.Context) error {
	data := url.Values{}
	data.Set("client_id", c.ClientID)
	data.Set("client_secret", c.ClientSecret)
	data.Set("refresh_token", c.RefreshToken)
	data.Set("grant_type", "refresh_token")

	req, err := http.NewRequestWithContext(ctx, "POST", "https://oauth2.googleapis.com/token", strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed: %s - %s", resp.Status, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	c.AccessToken = tokenResp.AccessToken
	log.Println("OAuth アクセストークンをリフレッシュしました")

	return nil
}

// loadOAuthCredentials はSecret Managerや環境変数から認証情報を読み込み
func loadOAuthCredentials(ctx context.Context) (*OAuthCredentials, error) {
	var refreshToken string

	// Secret Managerから取得を試行
	client, err := secretmanager.NewClient(ctx)
	if err == nil {
		defer client.Close()

		secretName := fmt.Sprintf("projects/%s/secrets/OAUTH_REFRESH_TOKEN/versions/latest", config.GCPProjectID)
		result, err := client.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
			Name: secretName,
		})
		if err == nil {
			refreshToken = strings.TrimSpace(string(result.Payload.Data))
			log.Println("OAuth refresh token loaded from Secret Manager")
		} else {
			log.Printf("Secret Manager読み込み失敗: %v", err)
		}
	}

	// 環境変数フォールバック
	if refreshToken == "" {
		refreshToken = strings.TrimSpace(os.Getenv("OAUTH_REFRESH_TOKEN"))
		if refreshToken == "" {
			refreshToken = strings.TrimSpace(os.Getenv("PHOTOS_REFRESH_TOKEN"))
		}
		if refreshToken != "" {
			log.Println("OAuth refresh token loaded from environment variable")
		}
	}

	if refreshToken == "" {
		return nil, fmt.Errorf("OAuth refresh token not found")
	}

	clientID := strings.TrimSpace(os.Getenv("OAUTH_CLIENT_ID"))
	clientSecret := strings.TrimSpace(os.Getenv("OAUTH_CLIENT_SECRET"))

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("OAUTH_CLIENT_ID or OAUTH_CLIENT_SECRET not set")
	}

	creds := &OAuthCredentials{
		RefreshToken: refreshToken,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	}

	// 初回アクセストークン取得
	if err := creds.refreshToken(ctx); err != nil {
		return nil, fmt.Errorf("failed to get initial access token: %w", err)
	}

	return creds, nil
}
