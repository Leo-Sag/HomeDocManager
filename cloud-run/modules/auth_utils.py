"""
Authentication Utilities for Google APIs
"""
import os
import logging
from google.cloud import secretmanager
from google.oauth2.credentials import Credentials
from google.auth.transport.requests import Request
from config.settings import GCP_PROJECT_ID

logger = logging.getLogger(__name__)

SECRET_NAME = "OAUTH_REFRESH_TOKEN"

def get_oauth_credentials() -> Credentials:
    """
    Retrieve OAuth credentials using Refresh Token from Secret Manager or Env.
    
    Returns:
        google.oauth2.credentials.Credentials: Valid (refreshed) credentials object.
    
    Raises:
        ValueError: If refresh token is missing.
    """
    try:
        # Try fetching from Secret Manager first
        client = secretmanager.SecretManagerServiceClient()
        name = f"projects/{GCP_PROJECT_ID}/secrets/{SECRET_NAME}/versions/latest"
        response = client.access_secret_version(request={"name": name})
        refresh_token = response.payload.data.decode('UTF-8').strip()
    except Exception as e:
        logger.debug(f"Secret Manager token fetch failed: {e}")
        # Fallback to Environment Variable
        refresh_token = os.getenv(SECRET_NAME)
        if refresh_token:
            refresh_token = refresh_token.strip()
    
    # Check for legacy environment variable name if new one is missing
    if not refresh_token:
        refresh_token = os.getenv('PHOTOS_REFRESH_TOKEN')
        if refresh_token:
            refresh_token = refresh_token.strip()
            logger.warning("Using legacy PHOTOS_REFRESH_TOKEN env var. Please update to OAUTH_REFRESH_TOKEN.")

    if not refresh_token:
        raise ValueError(f"OAuth refresh token not found in Secret Manager ({SECRET_NAME}) or Env")

    # Get Client ID and Secret from Env
    client_id = os.getenv('OAUTH_CLIENT_ID')
    if client_id:
        client_id = client_id.strip()
        
    client_secret = os.getenv('OAUTH_CLIENT_SECRET')
    if client_secret:
        client_secret = client_secret.strip()

    # Create Credentials object
    creds = Credentials(
        token=None,
        refresh_token=refresh_token,
        token_uri='https://oauth2.googleapis.com/token',
        client_id=client_id,
        client_secret=client_secret
    )
    
    # Refresh the token immediately to ensure validity
    try:
        creds.refresh(Request())
    except Exception as e:
        logger.error(f"Failed to refresh access token: {e}")
        raise

    return creds
