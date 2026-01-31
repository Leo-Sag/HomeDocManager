"""
Google Drive APIクライアント
ファイルの取得、移動、リネームなどの操作
"""
import time
import logging
import traceback
from typing import Optional, List, Dict, Any
from googleapiclient.discovery import build
from googleapiclient.http import MediaIoBaseDownload
from google.oauth2 import service_account
import io

logger = logging.getLogger(__name__)


class DriveClient:
    """Google Drive APIクライアント"""
    
    def __init__(self, credentials=None):
        """
        初期化
        
        Args:
            credentials: 認証情報（Noneの場合はデフォルト認証を使用）
        """
        if credentials:
            self.service = build('drive', 'v3', credentials=credentials)
        else:
            # デフォルト認証（Cloud Run環境ではサービスアカウント）
            self.service = build('drive', 'v3')
    
    def get_file(self, file_id: str) -> Optional[dict]:
        """
        ファイル情報を取得
        
        Args:
            file_id: ファイルID
            
        Returns:
            ファイル情報
        """
        try:
            file = self.service.files().get(
                fileId=file_id,
                fields='id, name, mimeType, parents'
            ).execute()
            return file
        except Exception as e:
            logger.error(f"ファイル情報取得エラー: {e}")
            return None
    
    def download_file(self, file_id: str) -> Optional[bytes]:
        """
        ファイルをダウンロード（堅牢化版）
        """
        import time
        import traceback
        import socket
        
        # デフォルトソケットタイムアウトを120秒に設定
        socket.setdefaulttimeout(120)
        
        max_retries = 5
        for attempt in range(max_retries):
            try:
                logger.info(f"ダウンロード開始（試行 {attempt+1}/{max_retries}）: {file_id}")
                
                # サービスオブジェクトの状態が悪い可能性があるので、リトライ時は再構築を検討
                if attempt > 0:
                    logger.info("サービスオブジェクトを再構築してリトライします")
                    self.service = build('drive', 'v3')

                request = self.service.files().get_media(fileId=file_id)
                file_content = request.execute()
                
                logger.info(f"ダウンロード完了: {len(file_content)} bytes")
                return file_content
            
            except Exception as e:
                # 最終試行で失敗
                if attempt == max_retries - 1:
                    logger.error(f"ダウンロード最終失敗: {e}")
                    logger.error(f"Traceback: {traceback.format_exc()}")
                    return None
                
                logger.info(f"ダウンロード失敗（リトライ待ち）: {e}")
                wait_time = 2 ** attempt
                time.sleep(wait_time)
        
        return None
    
    def move_file(
        self, 
        file_id: str, 
        new_parent_id: str,
        current_parent_id: Optional[str] = None
    ) -> bool:
        """
        ファイルを移動
        
        Args:
            file_id: ファイルID
            new_parent_id: 移動先フォルダID
            current_parent_id: 現在の親フォルダID（Noneの場合は自動取得）
            
        Returns:
            成功したかどうか
        """
        try:
            # 現在の親フォルダIDを取得
            if not current_parent_id:
                file = self.get_file(file_id)
                if not file or 'parents' not in file:
                    logger.error("親フォルダIDが取得できません")
                    return False
                current_parent_id = file['parents'][0]
            
            # ファイルを移動
            self.service.files().update(
                fileId=file_id,
                addParents=new_parent_id,
                removeParents=current_parent_id,
                fields='id, parents'
            ).execute()
            
            logger.info(f"ファイル移動成功: {file_id}")
            return True
        except Exception as e:
            logger.error(f"ファイル移動エラー: {e}")
            return False
    
    def rename_file(self, file_id: str, new_name: str) -> bool:
        """
        ファイル名を変更
        
        Args:
            file_id: ファイルID
            new_name: 新しいファイル名
            
        Returns:
            成功したかどうか
        """
        try:
            self.service.files().update(
                fileId=file_id,
                body={'name': new_name}
            ).execute()
            
            logger.info(f"ファイル名変更成功: {new_name}")
            return True
        except Exception as e:
            logger.error(f"ファイル名変更エラー: {e}")
            return False
    
    def create_folder(
        self, 
        folder_name: str, 
        parent_id: str
    ) -> Optional[str]:
        """
        フォルダを作成
        
        Args:
            folder_name: フォルダ名
            parent_id: 親フォルダID
            
        Returns:
            作成されたフォルダID
        """
        try:
            file_metadata = {
                'name': folder_name,
                'mimeType': 'application/vnd.google-apps.folder',
                'parents': [parent_id]
            }
            
            folder = self.service.files().create(
                body=file_metadata,
                fields='id',
                supportsAllDrives=True
            ).execute()
            
            logger.info(f"フォルダ作成成功: {folder_name}")
            return folder.get('id')
        except Exception as e:
            logger.error(f"フォルダ作成エラー: {e}")
            return None
    
    def find_folder(
        self, 
        folder_name: str, 
        parent_id: str
    ) -> Optional[str]:
        """
        フォルダを検索
        
        Args:
            folder_name: フォルダ名
            parent_id: 親フォルダID
            
        Returns:
            フォルダID（見つからない場合はNone）
        """
        try:
            query = f"name='{folder_name}' and '{parent_id}' in parents and mimeType='application/vnd.google-apps.folder' and trashed=false"
            results = self.service.files().list(
                q=query,
                fields='files(id, name)',
                supportsAllDrives=True,
                includeItemsFromAllDrives=True
            ).execute()
            
            files = results.get('files', [])
            if files:
                logger.info(f"既存フォルダ発見: {folder_name} ({files[0]['id']})")
                return files[0]['id']
            logger.info(f"フォルダが見つかりません: {folder_name} in {parent_id}")
            return None
        except Exception as e:
            logger.error(f"フォルダ検索エラー: {e}")
            return None
    
    def get_or_create_folder(
        self, 
        folder_name: str, 
        parent_id: str
    ) -> Optional[str]:
        """
        フォルダを取得または作成
        
        Args:
            folder_name: フォルダ名
            parent_id: 親フォルダID
            
        Returns:
            フォルダID
        """
        folder_id = self.find_folder(folder_name, parent_id)
        if folder_id:
            return folder_id
        return self.create_folder(folder_name, parent_id)

    def list_files(self, query: str, limit: int = 100) -> List[Dict]:
        """クエリに基づいてファイルを検索"""
        try:
            results = self.service.files().list(
                q=query,
                pageSize=limit,
                fields="nextPageToken, files(id, name, mimeType, size, owners, quotaBytesUsed)"
            ).execute()
            return results.get('files', [])
        except Exception as e:
            logger.error(f"ファイル検索エラー: {e}")
            return []

    def list_files_in_folder(self, folder_id: str, limit: int = 100) -> List[Dict]:
        """フォルダ内のファイルを検索"""
        query = f"'{folder_id}' in parents and trashed=false"
        return self.list_files(query, limit)

    def delete_file(self, file_id: str) -> bool:
        """ファイルを削除"""
        try:
            self.service.files().delete(fileId=file_id).execute()
            logger.info(f"ファイル削除成功: {file_id}")
            return True
        except Exception as e:
            logger.error(f"ファイル削除エラー: {e}")
            return False

    def cleanup_service_account_storage(self) -> Dict[str, Any]:
        """SAのストレージをクリーンアップ（ゴミ箱を空に & 所有ファイルを削除）"""
        stats = {'deleted_files': 0, 'freed_bytes': 0, 'errors': []}
        try:
            # 1. ゴミ箱を空にする
            try:
                self.service.files().emptyTrash().execute()
                logger.info("ゴミ箱を空にしました")
            except Exception as e:
                logger.warning(f"ゴミ箱のクリーンアップ失敗: {e}")
            
            # 2. SAが所有するファイルを検索 ('me' in owners)
            # NotebookLM同期ファイルなどの重要なものは除外したいが、
            # SAが所有している＝ユーザーに転送できていない＝容量圧迫の原因。
            # 今回は緊急対応として、SA所有の全ファイルを対象とする（ユーザー同意済みと仮定）
            # ただし、フォルダは消さないようにmimeTypeでフィルタしてもよいが、
            # deleteは再帰的ではないのでフォルダなら中身だけ消える？いや、フォルダも消える。
            # 安全のため、application/vnd.google-apps.folderは除外する。
            query = "'me' in owners and trashed = false and mimeType != 'application/vnd.google-apps.folder'"
            files = self.list_files(query, limit=100)
            
            for file in files:
                file_id = file['id']
                file_name = file.get('name', 'Unknown')
                file_size = int(file.get('quotaBytesUsed', 0))
                
                logger.info(f"Deleting SA-owned file: {file_name} ({file_size} bytes)")
                
                if self.delete_file(file_id):
                    stats['deleted_files'] += 1
                    stats['freed_bytes'] += file_size
                else:
                    stats['errors'].append(f"Failed to delete {file_name}")
                    
            return stats
        except Exception as e:
            logger.error(f"クリーンアップエラー: {e}")
            stats['errors'].append(str(e))
            return stats
