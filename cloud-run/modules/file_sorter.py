"""
FileSorterモジュール
GASのFileSorter.gsの機能をPythonに移植
"""
import logging
from typing import Dict, Optional
from datetime import datetime
from modules.ai_router import AIRouter
from modules.pdf_processor import PDFProcessor
from modules.drive_client import DriveClient
from modules.photos_client import PhotosClient
from config.settings import (
    FOLDER_IDS,
    CATEGORY_MAP,
    CHILD_ALIASES,
    SUB_CATEGORIES,
    SUPPORTED_MIME_TYPES
)

logger = logging.getLogger(__name__)


class FileSorter:
    """ファイル仕分けクラス"""
    
    def __init__(
        self,
        ai_router: AIRouter,
        pdf_processor: PDFProcessor,
        drive_client: DriveClient,
        photos_client: Optional[PhotosClient] = None
    ):
        """初期化"""
        self.ai_router = ai_router
        self.pdf_processor = pdf_processor
        self.drive_client = drive_client
        self.photos_client = photos_client
    
    def process_file(self, file_id: str) -> bool:
        """
        ファイルを処理
        
        Args:
            file_id: ファイルID
            
        Returns:
            成功したかどうか
        """
        try:
            # ファイル情報を取得
            file_info = self.drive_client.get_file(file_id)
            if not file_info:
                logger.error(f"ファイル情報取得失敗: {file_id}")
                return False
            
            file_name = file_info['name']
            mime_type = file_info['mimeType']
            
            logger.info(f"処理開始: {file_name}")
            print(f"[DEBUG] Processing started for: {file_name} ({file_id})", flush=True)
            
            # 対応ファイル形式をチェック
            if mime_type not in SUPPORTED_MIME_TYPES:
                logger.warning(f"非対応のファイル形式: {mime_type}")
                return False
            
            # ファイルをダウンロード
            print(f"[DEBUG] Downloading file...", flush=True)
            file_bytes = self.drive_client.download_file(file_id)
            if not file_bytes:
                logger.error(f"ファイルダウンロード失敗: {file_id}")
                return False
            print(f"[DEBUG] Download complete. Size: {len(file_bytes)} bytes", flush=True)
            
            # PDFの場合は画像に変換
            if self.pdf_processor.is_pdf(mime_type):
                print(f"[DEBUG] Converting PDF to image...", flush=True)
                images = self.pdf_processor.convert_pdf_to_images(file_bytes)
                if not images:
                    logger.error("PDF変換失敗")
                    return False
                # 最初のページを使用
                image_data = images[0]
                print(f"[DEBUG] PDF conversion complete.", flush=True)
            else:
                # 画像ファイルはそのまま使用
                image_data = file_bytes
                print(f"[DEBUG] Using original image.", flush=True)
            
            # Geminiで解析
            try:
                analysis_result = self._analyze_document(image_data, file_name)
                if not analysis_result:
                    logger.error("Gemini解析失敗")
                    return False
            except Exception as e:
                logger.error(f"Gemini解析致命的エラー: {e}")
                return False
            
            logger.info(f"解析結果: {analysis_result}")
            
            # 移動先フォルダを決定
            destination_folder_id = self._get_destination_folder(analysis_result)
            if not destination_folder_id:
                logger.error("移動先フォルダ決定失敗")
                return False
            
            # 新しいファイル名を生成
            new_file_name = self._generate_new_filename(
                analysis_result,
                file_name
            )
            
            # ファイルをリネーム
            print(f"[DEBUG] Renaming file to {new_file_name}...", flush=True)
            self.drive_client.rename_file(file_id, new_file_name)
            
            # ファイルを移動
            print(f"[DEBUG] Moving file to {destination_folder_id}...", flush=True)
            if not self.drive_client.move_file(file_id, destination_folder_id):
                logger.error(f"ファイル移動失敗: {file_id}")
                return False
            print(f"[DEBUG] Move complete.", flush=True)
            
            logger.info(f"処理完了: {file_name} → {new_file_name}")
            
            # Google Photosにアップロード（40-03または50の場合のみ）
            category = analysis_result.get('category', '')
            sub_category = analysis_result.get('sub_category', '')
            
            should_upload_to_photos = (
                category == '50_写真・その他' or
                (category == '40_子供・教育' and sub_category == '03_記録・作品・成績')
            )
            
            if self.photos_client and should_upload_to_photos:
                self._upload_to_photos(image_data, analysis_result)
            
            return True
            
        except Exception as e:
            logger.error(f"ファイル処理エラー: {e}")
            return False
    
    def _analyze_document(
        self,
        image_data: bytes,
        file_name: str
    ) -> Optional[Dict]:
        """ドキュメントを解析"""
        # 名寄せルールを文字列化
        aliases_str = '\n'.join([
            f"{name}: {', '.join(aliases)}"
            for name, aliases in CHILD_ALIASES.items()
        ])
        
        prompt = f"""
あなたは家庭内書類の整理アシスタントです。以下の画像を解析し、JSON形式で回答してください。

## お子様の名寄せルール
{aliases_str}

## 出力形式（必ずこのJSON形式で回答）
{{
  "category": "カテゴリ名（以下のいずれか）",
  "child_name": "お子様の名前（名寄せ後の正規名。複数または不明時は「共通・学校全般」）",
  "sub_category": "サブカテゴリ（categoryが40_子供・教育の場合のみ）",
  "is_photo": false,
  "date": "YYYYMMDD形式の日付",
  "summary": "要約（15文字以内、ファイル名に使用）",
  "confidence_score": 0.0
}}

## カテゴリ一覧
- 10_マネー・税務（銀行、保険、税金、請求書、領収書）
- 20_プロジェクト・資産（不動産、車、家電購入記録、修理記録）
- 30_ライフ・行政（役所、医療、年金、マイナンバー）
- 40_子供・教育（学校、塾、習い事のお便り）
- 50_写真・その他（書類ではない写真、分類不能なもの）
- 90_ライブラリ（家電の取扱説明書、ガイドブック、マニュアル類）

## サブカテゴリ（40_子供・教育の場合のみ使用）
- 01_お便り・スケジュール（行事予定、お知らせ）
- 02_提出・手続き・重要（提出書類、申込書）
- 03_記録・作品・成績（成績表、作品、賞状）

## 判断基準
- is_photoがtrueの場合は、categoryを「50_写真・その他」にしてください
- 日付が不明な場合は本日の日付を使用してください
- confidence_scoreは0.0〜1.0の範囲で、解析結果の信頼度を示してください

## ファイル名
{file_name}
"""
        
        return self.ai_router.analyze_document(image_data, prompt)
    
    def _get_destination_folder(self, result: Dict) -> Optional[str]:
        """移動先フォルダIDを取得"""
        category = result.get('category', '')
        
        # 写真の場合
        if result.get('is_photo', False) or category == '50_写真・その他':
            return FOLDER_IDS['PHOTO_OTHER']
        
        # 40_子供・教育の場合は年度フォルダ構造を作成
        if category == '40_子供・教育':
            return self._get_children_edu_folder(result)
        
        # その他のカテゴリ
        return CATEGORY_MAP.get(category, FOLDER_IDS['PHOTO_OTHER'])
    
    def _get_children_edu_folder(self, result: Dict) -> Optional[str]:
        """40_子供・教育用のフォルダ構造を作成"""
        base_folder_id = FOLDER_IDS['CHILDREN_EDU']
        
        # 子供名フォルダ
        child_name = result.get('child_name', '共通・学校全般')
        child_folder_id = self.drive_client.get_or_create_folder(
            child_name,
            base_folder_id
        )
        if not child_folder_id:
            return None
        
        # 年度フォルダ
        date_str = result.get('date', '')
        fiscal_year = self._get_fiscal_year(date_str)
        year_folder_id = self.drive_client.get_or_create_folder(
            f"{fiscal_year}年度",
            child_folder_id
        )
        if not year_folder_id:
            return None
        
        # サブカテゴリフォルダ
        sub_category = result.get('sub_category', '01_お便り・スケジュール')
        return self.drive_client.get_or_create_folder(
            sub_category,
            year_folder_id
        )
    
    def _get_fiscal_year(self, date_string: str) -> int:
        """日本の学校年度を取得（4月始まり）"""
        try:
            year = int(date_string[:4])
            month = int(date_string[4:6])
            
            # 1〜3月は前年度扱い
            if 1 <= month <= 3:
                return year - 1
            return year
        except:
            # パースエラーの場合は現在の年度
            now = datetime.now()
            year = now.year
            if now.month <= 3:
                return year - 1
            return year
    
    def _generate_new_filename(
        self,
        result: Dict,
        original_name: str
    ) -> str:
        """新しいファイル名を生成"""
        date = result.get('date', datetime.now().strftime('%Y%m%d'))
        summary = result.get('summary', 'document')
        
        # 拡張子を取得
        parts = original_name.split('.')
        extension = parts[-1] if len(parts) > 1 else 'pdf'
        
        return f"{date}_{summary}.{extension}"
    
    def _upload_to_photos(
        self,
        image_data: bytes,
        result: Dict
    ):
        """Google Photosにアップロード"""
        if not self.photos_client:
            return
        
        try:
            description = f"【{result.get('category', '')}】{result.get('date', '')}_{result.get('summary', '')}"
            url = self.photos_client.upload_image(image_data, description)
            if url:
                logger.info(f"Google Photosアップロード成功: {url}")
            else:
                logger.warning("Google Photosアップロード失敗")
        except Exception as e:
            logger.error(f"Google Photosアップロードエラー: {e}")
