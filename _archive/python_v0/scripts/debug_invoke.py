import os
import json
import base64
import subprocess
import requests
import sys

# Configuration
SERVICE_URL = "https://document-processor-333818874776.asia-northeast1.run.app"
TEST_FILE_ID = "1LIgzaK8u1jKW2-4X5veKDaFgCSlMxFNl"

def get_identity_token():
    try:
        result = subprocess.run(
            ["gcloud", "auth", "print-identity-token"],
            capture_output=True,
            text=True,
            check=True,
            shell=True 
        )
        return result.stdout.strip()
    except subprocess.CalledProcessError as e:
        print(f"Error getting identity token: {e}")
        return None

def invoke_service():
    token = get_identity_token()
    if not token:
        print("Failed to obtain identity token.")
        return

    # Use /test endpoint instead of root
    url = f"{SERVICE_URL}/test"
    
    # Simple JSON payload for /test
    payload = {"file_id": TEST_FILE_ID}

    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

    print(f"Invoking {url} with file_id: {TEST_FILE_ID}")
    
    try:
        response = requests.post(url, json=payload, headers=headers, timeout=120)
        print(f"Status Code: {response.status_code}")
        print("Response Body:")
        print(response.text)
    except Exception as e:
        print(f"Request failed: {e}")

if __name__ == "__main__":
    invoke_service()
