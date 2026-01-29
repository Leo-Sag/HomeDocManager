"""
Google Photos API OAuth 2.0セットアップスクリプト
初回のみローカルPCで実行してリフレッシュトークンを取得
"""
import os
import sys
from dotenv import load_dotenv, find_dotenv
from google_auth_oauthlib.flow import InstalledAppFlow
from google.cloud import secretmanager

SCOPES = ['https://www.googleapis.com/auth/photoslibrary.appendonly']


def main():
    """メイン処理"""
    # .envから環境変数を読み込む
    load_dotenv(find_dotenv(usecwd=True))

    # GCPプロジェクトIDを環境変数から取得
    gcp_project_id = os.getenv('GCP_PROJECT_ID')
    if not gcp_project_id:
        print("エラー: GCP_PROJECT_ID環境変数が設定されていません")
        sys.exit(1)
    
    # クライアントシークレットファイルの確認
    client_secret_file = 'client_secret.json'
    if not os.path.exists(client_secret_file):
        print(f"エラー: {client_secret_file}が見つかりません")
        print("GCPコンソールからOAuth 2.0クライアントIDを作成し、JSONをダウンロードしてください")
        sys.exit(1)
    
    print("Google Photos API OAuth 2.0認証を開始します...")
    print("ブラウザが開きますので、Googleアカウントでログインしてください")
    
    # OAuth 2.0フロー実行
    flow = InstalledAppFlow.from_client_secrets_file(
        client_secret_file,
        SCOPES
    )
    creds = flow.run_local_server(port=8080)
    
    print(f"\nリフレッシュトークン取得成功:")
    print(f"{creds.refresh_token}\n")
    
    # Secret Managerに保存するか確認
    save_to_secret = input("Secret Managerに保存しますか？ (y/n): ")
    
    if save_to_secret.lower() == 'y':
        try:
            client = secretmanager.SecretManagerServiceClient()
            parent = f"projects/{gcp_project_id}"
            
            # シークレット作成
            try:
                secret = client.create_secret(
                    request={
                        "parent": parent,
                        "secret_id": "PHOTOS_REFRESH_TOKEN",
                        "secret": {"replication": {"automatic": {}}}
                    }
                )
                print(f"シークレット作成成功: {secret.name}")
            except Exception as e:
                print(f"シークレット作成スキップ（既に存在する可能性）: {e}")
                secret_name = f"{parent}/secrets/PHOTOS_REFRESH_TOKEN"
            
            # バージョン追加
            client.add_secret_version(
                request={
                    "parent": f"{parent}/secrets/PHOTOS_REFRESH_TOKEN",
                    "payload": {"data": creds.refresh_token.encode('UTF-8')}
                }
            )
            
            print("リフレッシュトークンをSecret Managerに保存しました")
            print("\n次のステップ:")
            print("1. .envファイルにOAUTH_CLIENT_IDとOAUTH_CLIENT_SECRETを設定")
            print("2. Cloud Runにデプロイ")
            
        except Exception as e:
            print(f"Secret Manager保存エラー: {e}")
            print("\n手動でSecret Managerに保存してください:")
            print(f"シークレット名: PHOTOS_REFRESH_TOKEN")
            print(f"値: {creds.refresh_token}")
    else:
        print("\n.envファイルに以下を追加してください:")
        print(f"PHOTOS_REFRESH_TOKEN={creds.refresh_token}")


if __name__ == '__main__':
    main()
