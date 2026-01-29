"""
NotebookLM同期モジュール
処理済みファイルのOCRテキストを年度別累積ドキュメントに追記する
Drive APIのみを使用してGoogleドキュメントを作成・更新
"""
import logging
from typing import Optional
from datetime import datetime
from io import BytesIO
from googleapiclient.http import MediaIoBaseUpload
from modules.drive_client import DriveClient
from config.settings import (
    FOLDER_IDS,
    NOTEBOOKLM_SYNC_CATEGORIES
)

logger = logging.getLogger(__name__)


class NotebookLMSync:
    """NotebookLM用シャドウドキュメント同期クラス"""
    
    # 同期済みマーカー（Driveプロパティに設定）
    PROCESSED_MARKER = 'notebooklm_synced'
    
    def __init__(self, drive_client: DriveClient):
        """
        初期化
        
        Args:
            drive_client: DriveClientインスタンス
        """
        self.drive_client = drive_client
    
    def should_sync(self, category: str) -> bool:
        """
        このカテゴリが同期対象かどうかをチェック
        
        Args:
            category: カテゴリ名
            
        Returns:
            同期対象の場合True
        """
        return category in NOTEBOOKLM_SYNC_CATEGORIES
    
    def sync_file(
        self,
        file_id: str,
        file_name: str,
        category: str,
        ocr_text: str,
        date_str: str,
        fiscal_year: int
    ) -> bool:
        """
        ファイルのOCRテキストを累積ドキュメントに追記
        
        Args:
            file_id: 元ファイルのID
            file_name: 元ファイル名
            category: カテゴリ名
            ocr_text: OCRで抽出されたテキスト
            date_str: YYYYMMDD形式の日付
            fiscal_year: 年度
            
        Returns:
            成功したかどうか
        """
        if not self.should_sync(category):
            logger.info(f"カテゴリ {category} は同期対象外です")
            return False
        
        try:
            # 日付をフォーマット
            formatted_date = self._format_date(date_str)
            
            # 累積ドキュメントを取得または作成
            doc_id = self._get_or_create_accumulated_doc(fiscal_year)
            if not doc_id:
                logger.error(f"累積ドキュメント取得/作成失敗: {fiscal_year}年度")
                return False
            
            # ドキュメントに追記（カテゴリ名を含める）
            entry_text = self._format_entry(formatted_date, file_name, file_id, ocr_text, category)
            if not self._append_to_doc(doc_id, entry_text):
                logger.error(f"ドキュメント追記失敗: {doc_id}")
                return False
            
            # 元ファイルに同期済みマーカーを設定
            self._mark_as_synced(file_id)
            
            logger.info(f"NotebookLM同期完了: {file_name} → {fiscal_year}年度_全記録")
            return True
            
        except Exception as e:
            logger.error(f"NotebookLM同期エラー: {e}")
            return False
    
    def _format_date(self, date_str: str) -> str:
        """YYYYMMDD形式をYYYY/MM/DD形式に変換"""
        if not date_str or len(date_str) != 8:
            return datetime.now().strftime('%Y/%m/%d')
        return f"{date_str[:4]}/{date_str[4:6]}/{date_str[6:8]}"
    
    def _format_entry(
        self,
        formatted_date: str,
        file_name: str,
        file_id: str,
        ocr_text: str,
        category: str
    ) -> str:
        """エントリテキストをフォーマット（カテゴリ名を含む）"""
        file_url = f"https://drive.google.com/file/d/{file_id}/view"
        
        entry = f"""

========================================
📄 {formatted_date} - [{category}] {file_name}
🔗 {file_url}
========================================

{ocr_text}

"""
        return entry
    
    def _get_or_create_accumulated_doc(self, fiscal_year: int) -> Optional[str]:
        """
        年度別統合ドキュメントを取得または作成
        全カテゴリを1つのドキュメントに統合（NotebookLM用）
        
        Args:
            fiscal_year: 年度
            
        Returns:
            ドキュメントID
        """
        sync_folder_id = FOLDER_IDS.get('NOTEBOOKLM_SYNC')
        if not sync_folder_id:
            logger.error("NOTEBOOKLM_SYNCフォルダIDが設定されていません")
            return None
        
        # ドキュメント名（年度のみ、全カテゴリ統合）
        doc_name = f"{fiscal_year}年度_全記録"
        
        # 既存のドキュメントを検索（同期フォルダ直下）
        doc_id = self._find_doc_by_name(doc_name, sync_folder_id)
        if doc_id:
            return doc_id
        
        # 新規作成（同期フォルダ直下）- Drive APIを使用
        return self._create_unified_doc(doc_name, sync_folder_id, fiscal_year)
    
    def _find_doc_by_name(self, doc_name: str, parent_id: str) -> Optional[str]:
        """
        フォルダ内でGoogleドキュメントを名前で検索
        
        Args:
            doc_name: ドキュメント名
            parent_id: 親フォルダID
            
        Returns:
            ドキュメントID（見つからない場合はNone）
        """
        try:
            query = (
                f"name='{doc_name}' and '{parent_id}' in parents "
                f"and mimeType='application/vnd.google-apps.document' and trashed=false"
            )
            results = self.drive_client.service.files().list(
                q=query,
                fields='files(id, name)'
            ).execute()
            
            files = results.get('files', [])
            if files:
                return files[0]['id']
            return None
        except Exception as e:
            logger.error(f"ドキュメント検索エラー: {e}")
            return None
    
    def _create_unified_doc(
        self,
        doc_name: str,
        parent_id: str,
        fiscal_year: int
    ) -> Optional[str]:
        """
        Drive APIを使用して新しい統合ドキュメントを作成（全カテゴリ用）
        
        Args:
            doc_name: ドキュメント名
            parent_id: 親フォルダID
            fiscal_year: 年度
            
        Returns:
            作成されたドキュメントID
        """
        try:
            # ヘッダーテキスト
            header_text = f"""# {fiscal_year}年度 全記録

このドキュメントは NotebookLM 用に自動生成された書類OCRテキストの統合ファイルです。
各エントリには [カテゴリ名] が付与されています。

---

"""
            # Drive APIでGoogleドキュメントを作成
            file_metadata = {
                'name': doc_name,
                'mimeType': 'application/vnd.google-apps.document',
                'parents': [parent_id]
            }
            
            # 空のドキュメントを作成
            doc = self.drive_client.service.files().create(
                body=file_metadata,
                fields='id'
            ).execute()
            
            doc_id = doc.get('id')
            
            # ヘッダーを追加
            self._append_to_doc(doc_id, header_text)
            
            # オーナー権限をユーザーに転送（サービスアカウントの容量制限回避）
            # NOTE: transferOwnership=True は role='owner' の場合に必須
            from config.settings import NOTEBOOKLM_OWNER_EMAIL
            if NOTEBOOKLM_OWNER_EMAIL:
                try:
                    self.drive_client.service.permissions().create(
                        fileId=doc_id,
                        body={
                            'role': 'owner',
                            'type': 'user',
                            'emailAddress': NOTEBOOKLM_OWNER_EMAIL
                        },
                        transferOwnership=True
                    ).execute()
                    logger.info(f"ドキュメントのオーナー権限を転送しました: {NOTEBOOKLM_OWNER_EMAIL}")
                except Exception as e:
                    logger.warning(f"オーナー権限の転送に失敗しました（容量制限に注意）: {e}")

            logger.info(f"統合ドキュメント作成: {doc_name}")
            return doc_id
            
        except Exception as e:
            logger.error(f"ドキュメント作成エラー: {e}")
            return None
    
    def _append_to_doc(self, doc_id: str, text: str) -> bool:
        """
        ドキュメントの末尾にテキストを追記
        Drive APIのexport/updateを使用
        
        Args:
            doc_id: ドキュメントID
            text: 追記するテキスト
            
        Returns:
            成功したかどうか
        """
        try:
            # 現在のドキュメント内容をテキストとしてエクスポート
            current_content = self.drive_client.service.files().export(
                fileId=doc_id,
                mimeType='text/plain'
            ).execute()
            
            # 既存の内容がbytesの場合はデコード
            if isinstance(current_content, bytes):
                current_content = current_content.decode('utf-8')
            
            # 新しい内容を追加
            new_content = current_content + text
            
            # テキストファイルとしてアップロード（Googleドキュメントに変換）
            media = MediaIoBaseUpload(
                BytesIO(new_content.encode('utf-8')),
                mimetype='text/plain',
                resumable=True
            )
            
            self.drive_client.service.files().update(
                fileId=doc_id,
                media_body=media
            ).execute()
            
            return True
            
        except Exception as e:
            logger.error(f"ドキュメント追記エラー: {e}")
            return False
    
    def _mark_as_synced(self, file_id: str):
        """
        ファイルを同期済みとしてマーク
        
        Args:
            file_id: ファイルID
        """
        try:
            # Drive APIのpropertiesを使用
            self.drive_client.service.files().update(
                fileId=file_id,
                body={
                    'properties': {
                        self.PROCESSED_MARKER: 'true'
                    }
                }
            ).execute()
        except Exception as e:
            logger.warning(f"同期マーキングエラー: {e}")
    
    def is_already_synced(self, file_id: str) -> bool:
        """
        ファイルが既に同期済みかチェック
        
        Args:
            file_id: ファイルID
            
        Returns:
            同期済みの場合True
        """
        try:
            file = self.drive_client.service.files().get(
                fileId=file_id,
                fields='properties'
            ).execute()
            
            properties = file.get('properties', {})
            return properties.get(self.PROCESSED_MARKER) == 'true'
            
        except Exception as e:
            logger.warning(f"同期状態チェックエラー: {e}")
            return False
