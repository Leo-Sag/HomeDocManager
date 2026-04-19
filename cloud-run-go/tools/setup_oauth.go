package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"

	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func main() {
	ctx := context.Background()

	// GCP Project IDを取得
	projectID := os.Getenv("GCP_PROJECT_ID")
	if projectID == "" {
		log.Fatal("Error: GCP_PROJECT_ID environment variable is not set")
	}

	// client_secret.jsonを読み込み
	b, err := os.ReadFile("client_secret.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v\n", err)
	}

	// OAuth2設定
	// Note: auth/drive (Restricted) を除去し、Sensitive以下のスコープのみ使用
	// これにより CASA第三者監査なしで同意画面を本番公開可能
	config, err := google.ConfigFromJSON(b,
		"https://www.googleapis.com/auth/photoslibrary.appendonly", // Google Photos (appendonly)
		"https://www.googleapis.com/auth/calendar.events",          // Google Calendar
		"https://www.googleapis.com/auth/tasks",                    // Google Tasks
		"https://www.googleapis.com/auth/documents",                // Google Docs (create/edit)
		"https://www.googleapis.com/auth/drive.file",               // Drive: NotebookLM doc creation + About.Get (Recommended, not Restricted)
	)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	// Redirect URIを127.0.0.1に設定（より安定的な認証のため）
	config.RedirectURL = "http://127.0.0.1:8080/"

	fmt.Println("Starting OAuth 2.0 authentication for Photos, Calendar, and Tasks...")
	fmt.Println("A browser window will open. Please login with your Google Account.")
	fmt.Println()

	// 認証URL生成
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline, oauth2.ApprovalForce)

	// auth.html ファイルを作成してリンクを書き込む
	htmlContent := fmt.Sprintf(`<html><body><h1>OAuth 認証</h1><p>以下のリンクをクリックして認証を開始してください：</p><a href="%s" style="font-size: 20px;">👉 ここをクリックして認証を開始</a></body></html>`, authURL)
	os.WriteFile("auth.html", []byte(htmlContent), 0644)

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🔑  重要: 以下のいずれかの方法で認証を開始してください")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Printf("\n1. このファイルを開く: k:/.../cloud-run-go/auth.html\n")
	fmt.Printf("2. 以下のURLを【最後まですべて】コピーしてブラウザで開く:\n\n%s\n\n", authURL)
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("\n⚠️  注意: 「このアプリは確認されていません」と表示された場合は、")
	fmt.Println("   「詳細」->「... へ移動（安全ではない）」をクリックしてください。")
	fmt.Println()

	// ローカルサーバーを起動して認証コードを受け取る
	codeChan := make(chan string)
	server := &http.Server{Addr: ":8080"}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			fmt.Fprintf(w, "Error: No code in the response")
			return
		}
		codeChan <- code
		fmt.Fprintf(w, "<h1>認証成功！</h1><p>このウィンドウを閉じてターミナルに戻ってください。</p>")
		go func() {
			server.Shutdown(context.Background())
		}()
	})

	go func() {
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// ブラウザを開く（Windows, Mac, Linuxに対応）
	openBrowser(authURL)

	// 認証コードを待つ
	code := <-codeChan
	server.Shutdown(ctx)

	fmt.Println("\n認証コードを受信しました。トークンを取得中...")

	// トークンを取得
	tok, err := config.Exchange(ctx, code)
	if err != nil {
		log.Fatalf("Unable to retrieve token: %v", err)
	}

	if tok.RefreshToken == "" {
		log.Fatal("Error: No refresh token returned. Please revoke access and try again.")
	}

	fmt.Println("\n✅ リフレッシュトークンの取得に成功しました！")
	fmt.Printf("\nRefresh Token:\n%s\n\n", tok.RefreshToken)

	// Secret Managerに自動保存
	fmt.Println("Secret Managerに保存中...")
	if err := saveToSecretManager(ctx, projectID, tok.RefreshToken); err != nil {
		log.Printf("Error saving to Secret Manager: %v", err)
		fmt.Println("\n⚠️  自動保存に失敗しました。手動でSecret Managerに保存してください:")
		printManualInstructions(tok.RefreshToken)
	} else {
		fmt.Println("✅ Secret Managerへの保存が完了しました！（OAUTH_REFRESH_TOKEN）")
		fmt.Println("\n次のステップ:")
		fmt.Println("1. Cloud Runを再起動して新しいトークンを反映:")
		fmt.Printf("   gcloud run services update %s --region=asia-northeast1 --project=%s --no-traffic\n", "homedocmanager-go", projectID)
		fmt.Printf("   gcloud run services update-traffic %s --region=asia-northeast1 --project=%s --to-latest\n", "homedocmanager-go", projectID)
		fmt.Println("2. admin/info でOAuthユーザーが表示されることを確認")
	}

	// トークン全体をJSONで保存（オプション）
	tokenJSON, _ := json.MarshalIndent(tok, "", "  ")
	os.WriteFile("token.json", tokenJSON, 0600)
	fmt.Println("\n📄 トークン全体を token.json に保存しました")
}

// saveToSecretManager はSecret Managerにトークンを保存
func saveToSecretManager(ctx context.Context, projectID, refreshToken string) error {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create secretmanager client: %w", err)
	}
	defer client.Close()

	secretID := "OAUTH_REFRESH_TOKEN"
	parent := fmt.Sprintf("projects/%s", projectID)

	// Secretを作成（既に存在する場合はスキップ）
	createReq := &secretmanagerpb.CreateSecretRequest{
		Parent:   parent,
		SecretId: secretID,
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	}

	_, err = client.CreateSecret(ctx, createReq)
	if err != nil {
		// 既に存在する場合はエラーを無視
		fmt.Println("Secret already exists (or creation failed), adding new version...")
	}

	// バージョンを追加
	addVersionReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: fmt.Sprintf("%s/secrets/%s", parent, secretID),
		Payload: &secretmanagerpb.SecretPayload{
			Data: []byte(refreshToken),
		},
	}

	_, err = client.AddSecretVersion(ctx, addVersionReq)
	if err != nil {
		return fmt.Errorf("failed to add secret version: %w", err)
	}

	return nil
}

// printManualInstructions は手動でのSecret Manager保存手順を表示
func printManualInstructions(refreshToken string) {
	fmt.Println("\nコマンド:")
	fmt.Printf("gcloud secrets create OAUTH_REFRESH_TOKEN --data-file=- <<< \"%s\"\n", refreshToken)
	fmt.Println("\nまたは:")
	fmt.Println("1. GCP Console > Secret Manager を開く")
	fmt.Println("2. 「シークレットを作成」をクリック")
	fmt.Println("3. 名前: OAUTH_REFRESH_TOKEN")
	fmt.Printf("4. シークレットの値: %s\n", refreshToken)
}

// openBrowser はデフォルトブラウザでURLを開く
func openBrowser(url string) {
	var err error
	switch os.Getenv("OS") {
	case "Windows_NT":
		// Windowsではstart コマンドがURL中の&を誤解釈するため、
		// rundll32経由でブラウザを起動する
		err = runCommand("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		// Mac, Linux
		err = runCommand("open", url)
		if err != nil {
			err = runCommand("xdg-open", url)
		}
	}
	if err != nil {
		fmt.Println("ブラウザを自動で開けませんでした。上記のURLを手動でブラウザにコピーしてください。")
	}
}

// runCommand はコマンドを実行
func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Start()
}
