"""
Google Photos, Calendar, and Tasks API OAuth 2.0 setup script
Run this locally to obtain a refresh token for all required services.
"""
import os
import sys
from dotenv import load_dotenv, find_dotenv
from google_auth_oauthlib.flow import InstalledAppFlow
from google.cloud import secretmanager

# Scopes for Photos, Calendar, and Tasks
SCOPES = [
    'https://www.googleapis.com/auth/photoslibrary.appendonly',
    'https://www.googleapis.com/auth/calendar',
    'https://www.googleapis.com/auth/tasks'
]


def main():
    """Main execution"""
    # Load env vars
    load_dotenv(find_dotenv(usecwd=True))

    # Get GCP Project ID
    gcp_project_id = os.getenv('GCP_PROJECT_ID')
    if not gcp_project_id:
        print("Error: GCP_PROJECT_ID environment variable is not set")
        sys.exit(1)
    
    # Check client secret file
    client_secret_file = 'client_secret.json'
    if not os.path.exists(client_secret_file):
        print(f"Error: {client_secret_file} not found")
        print("Please create an OAuth 2.0 Client ID in GCP Console and download the JSON")
        sys.exit(1)
    
    print("Starting OAuth 2.0 authentication for Photos, Calendar, and Tasks...")
    print("A browser window will open. Please login with your Google Account.")
    
    # Run OAuth 2.0 flow
    flow = InstalledAppFlow.from_client_secrets_file(
        client_secret_file,
        SCOPES
    )
    creds = flow.run_local_server(port=8080)
    
    print(f"\nRefresh Token obtained successfully:")
    print(f"{creds.refresh_token}\n")
    
    # Ask to save to Secret Manager
    save_to_secret = input("Save to Secret Manager? (y/n): ")
    
    SECRET_NAME = "OAUTH_REFRESH_TOKEN"

    if save_to_secret.lower() == 'y':
        try:
            client = secretmanager.SecretManagerServiceClient()
            parent = f"projects/{gcp_project_id}"
            
            # Create Secret
            try:
                secret = client.create_secret(
                    request={
                        "parent": parent,
                        "secret_id": SECRET_NAME,
                        "secret": {"replication": {"automatic": {}}}
                    }
                )
                print(f"Secret created: {secret.name}")
            except Exception as e:
                print(f"Secret creation skipped (might already exist): {e}")
            
            # Add Version
            client.add_secret_version(
                request={
                    "parent": f"{parent}/secrets/{SECRET_NAME}",
                    "payload": {"data": creds.refresh_token.encode('UTF-8')}
                }
            )
            
            print(f"Refresh token saved to Secret Manager as {SECRET_NAME}")
            print("\nNext steps:")
            print("1. Update OAUTH_CLIENT_ID and OAUTH_CLIENT_SECRET in .env")
            print("2. Deploy to Cloud Run")
            
        except Exception as e:
            print(f"Error saving to Secret Manager: {e}")
            print("\nPlease manually save to Secret Manager:")
            print(f"Secret Name: {SECRET_NAME}")
            print(f"Value: {creds.refresh_token}")
    else:
        print("\nPlease add the following to your .env file:")
        print(f"{SECRET_NAME}={creds.refresh_token}")


if __name__ == '__main__':
    main()
