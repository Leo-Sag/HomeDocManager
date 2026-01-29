"""
PDF処理モジュール
poppler-utilsを使用したPDF→画像変換
"""
import io
import logging
from typing import List
from pdf2image import convert_from_bytes
from PIL import Image

logger = logging.getLogger(__name__)


class PDFProcessor:
    """PDF処理クラス"""
    
    def convert_pdf_to_images(
        self, 
        pdf_bytes: bytes,
        dpi: int = 200
    ) -> List[bytes]:
        """
        PDFを画像に変換
        
        Args:
            pdf_bytes: PDFバイナリデータ
            dpi: 解像度（200-300dpiを推奨）
            
        Returns:
            画像バイナリデータのリスト
        """
        try:
            print(f"[DEBUG] Calling convert_from_bytes(dpi={dpi})...", flush=True)
            # PDFを画像に変換（メモリ上で処理）
            # thread_count=1 にしてデッドロック回避
            images = convert_from_bytes(pdf_bytes, dpi=dpi, thread_count=1)
            print(f"[DEBUG] convert_from_bytes finished. Found {len(images)} pages.", flush=True)
            
            # 各ページをJPEGバイナリに変換
            image_bytes_list = []
            for i, image in enumerate(images):
                print(f"[DEBUG] Processing page {i+1}...", flush=True)
                img_byte_arr = io.BytesIO()
                image.save(img_byte_arr, format='JPEG', quality=85)
                image_bytes_list.append(img_byte_arr.getvalue())
                logger.info(f"ページ {i+1}/{len(images)} を変換完了")
            
            return image_bytes_list
            
        except Exception as e:
            print(f"[DEBUG] PDF conversion error: {e}", flush=True)
            logger.error(f"PDF変換エラー: {e}")
            return []
    
    def is_pdf(self, mime_type: str) -> bool:
        """PDFファイルかチェック"""
        return mime_type == 'application/pdf'
