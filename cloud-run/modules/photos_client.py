"""
Google Photos APIクライアント
OAuth 2.0リフレッシュトークンを使用した認証と2段階アップロード
"""
import os
import logging
import requests
from typing import Optional
from google.cloud import secretmanager
from google.oauth2.credentials import Credentials
from google.auth.transport.requests import Request
from config.settings import GCP_PROJECT_ID, SECRET_PHOTOS_REFRESH_TOKEN

logger = logging.getLogger(__name__)


class PhotosClient:
    """Google Photos APIクライアント"""
    
    def __init__(self):
        """初期化"""
        self.credentials = self._get_credentials()
        
    def _get_credentials(self) -> Credentials:
        """Secret Managerからリフレッシュトークンを取得して認証"""
        try:
            client = secretmanager.SecretManagerServiceClient()
            name = f"projects/{GCP_PROJECT_ID}/secrets/{SECRET_PHOTOS_REFRESH_TOKEN}/versions/latest"
            response = client.access_secret_version(request={"name": name})
            refresh_token = response.payload.data.decode('UTF-8').strip()
        except Exception as e:
            logger.error(f"Secret Managerトークン取得エラー: {e}")
            # フォールバック: 環境変数から取得
            refresh_token = os.getenv('PHOTOS_REFRESH_TOKEN')
            if refresh_token:
                refresh_token = refresh_token.strip()
            
            if not refresh_token:
                raise ValueError("Photos refresh tokenが見つかりません")
        
        # クライアントIDとシークレットもサニタイズ
        client_id = os.getenv('OAUTH_CLIENT_ID')
        if client_id:
            client_id = client_id.strip()
            
        client_secret = os.getenv('OAUTH_CLIENT_SECRET')
        if client_secret:
            client_secret = client_secret.strip()

        # リフレッシュトークンから認証情報を作成
        creds = Credentials(
            token=None,
            refresh_token=refresh_token,
            token_uri='https://oauth2.googleapis.com/token',
            client_id=client_id,
            client_secret=client_secret
        )
        
        # トークンをリフレッシュ
        creds.refresh(Request())
        return creds
    
    def upload_image(
        self, 
        image_bytes: bytes, 
        description: str
    ) -> Optional[str]:
        """
        画像をGoogle Photosにアップロード（2段階プロトコル）
        
        Args:
            image_bytes: 画像バイナリデータ
            description: 説明文（メタデータ）
            
        Returns:
            アップロードされた画像のURL
        """
        try:
            # 第一段階: バイトアップロード
            upload_token = self._upload_bytes(image_bytes)
            if not upload_token:
                return None
            
            # 第二段階: メディアアイテム作成
            return self._create_media_item(upload_token, description)
            
        except Exception as e:
            logger.error(f"Google Photosアップロードエラー: {e}")
            return None
    
    def _upload_bytes(self, image_bytes: bytes) -> Optional[str]:
        """第一段階: バイトアップロード"""
        url = 'https://photoslibrary.googleapis.com/v1/uploads'
        headers = {
            'Authorization': f'Bearer {self.credentials.token}',
            'Content-Type': 'application/octet-stream',
            'X-Goog-Upload-Content-Type': 'image/jpeg',
            'X-Goog-Upload-Protocol': 'raw'
        }
        
        response = requests.post(url, headers=headers, data=image_bytes)
        if response.status_code == 200:
            logger.info("バイトアップロード成功")
            return response.text  # Upload Token
        else:
            logger.error(f"バイトアップロード失敗: {response.status_code} - {response.text}")
            return None
    
    def _create_media_item(
        self, 
        upload_token: str, 
        description: str
    ) -> Optional[str]:
        """第二段階: メディアアイテム作成"""
        url = 'https://photoslibrary.googleapis.com/v1/mediaItems:batchCreate'
        headers = {
            'Authorization': f'Bearer {self.credentials.token}',
            'Content-Type': 'application/json'
        }
        
        payload = {
            'newMediaItems': [{
                'description': description,
                'simpleMediaItem': {
                    'uploadToken': upload_token
                }
            }]
        }
        
        response = requests.post(url, headers=headers, json=payload)
        if response.status_code == 200:
            result = response.json()
            media_item = result['newMediaItemResults'][0]['mediaItem']
            logger.info(f"メディアアイテム作成成功: {media_item['id']}")
            return media_item['productUrl']
        else:
            logger.error(f"メディアアイテム作成失敗: {response.status_code} - {response.text}")
            return None
