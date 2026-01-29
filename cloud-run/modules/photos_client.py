"""
Google Photos API Client
Authentication and 2-step upload using OAuth 2.0 Refresh Token.
"""
import logging
import requests
from typing import Optional
from modules.auth_utils import get_oauth_credentials

logger = logging.getLogger(__name__)


class PhotosClient:
    """Google Photos API Client"""
    
    def __init__(self):
        """Initialize"""
        self.credentials = get_oauth_credentials()
    
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
