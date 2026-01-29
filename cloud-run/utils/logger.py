"""
ロギング設定モジュール
"""
import logging
import sys


def setup_logging(level=logging.INFO):
    """
    ロギングを設定
    
    Args:
        level: ログレベル
    """
    logging.basicConfig(
        level=level,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        handlers=[
            logging.StreamHandler(sys.stdout)
        ]
    )
    
    # Google Cloud Loggingとの統合
    # Google Cloud Loggingとの統合（Cloud Runでは標準出力で十分なため無効化）
    # try:
    #     import google.cloud.logging
    #     client = google.cloud.logging.Client()
    #     client.setup_logging()
    # except Exception as e:
    #     logging.warning(f"Google Cloud Logging統合失敗: {e}")
