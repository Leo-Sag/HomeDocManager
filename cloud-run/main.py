"""
Cloud Runメインアプリケーション
Pub/Subトリガーを受けてドキュメント処理を実行
"""
import os
import json
import logging
import base64
from flask import Flask, request
from utils.logger import setup_logging
from modules.ai_router import AIRouter
from modules.pdf_processor import PDFProcessor
from modules.photos_client import PhotosClient
from modules.drive_client import DriveClient
from modules.calendar_client import CalendarClient
from modules.tasks_client import TasksClient
from modules.file_sorter import FileSorter

# ロギング設定
setup_logging()
logger = logging.getLogger(__name__)

app = Flask(__name__)

# モジュール初期化（遅延初期化）
ai_router = None
pdf_processor = None
photos_client = None
drive_client = None
calendar_client = None
tasks_client = None
file_sorter = None


def init_modules():
    """モジュールを初期化"""
    global ai_router, pdf_processor, photos_client, drive_client, calendar_client, tasks_client, file_sorter
    
    if ai_router is None:
        logger.info("モジュールを初期化中...")
        ai_router = AIRouter()
        pdf_processor = PDFProcessor()
        drive_client = DriveClient()
        
        # Google Photos APIは環境変数が設定されている場合のみ
        try:
            photos_client = PhotosClient()
            logger.info("Google Photos APIクライアント初期化成功")
        except Exception as e:
            logger.warning(f"Google Photos APIクライアント初期化失敗: {e}")
            photos_client = None

        # Google Calendar API
        try:
            calendar_client = CalendarClient()
            logger.info("Google Calendar APIクライアント初期化成功")
        except Exception as e:
            logger.warning(f"Google Calendar APIクライアント初期化失敗: {e}")
            calendar_client = None

        # Google Tasks API
        try:
            tasks_client = TasksClient()
            logger.info("Google Tasks APIクライアント初期化成功")
        except Exception as e:
            logger.warning(f"Google Tasks APIクライアント初期化失敗: {e}")
            tasks_client = None
        
        file_sorter = FileSorter(
            ai_router,
            pdf_processor,
            drive_client,
            photos_client,
            calendar_client,
            tasks_client
        )
        logger.info("モジュール初期化完了")


@app.route('/', methods=['POST'])
def handle_pubsub():
    """Pub/Subトリガーハンドラ"""
    try:
        # モジュール初期化
        init_modules()
        
        envelope = request.get_json()
        if not envelope:
            logger.error("Invalid Pub/Sub message format")
            return 'Bad Request', 400
        
        # Pub/Subメッセージをデコード
        pubsub_message = envelope.get('message', {})
        data = base64.b64decode(pubsub_message.get('data', '')).decode('utf-8')
        message_data = json.loads(data)
        
        # ファイルIDを取得
        file_id = message_data.get('file_id')
        if not file_id:
            logger.error("file_id not found in message")
            return 'Bad Request', 400
        
        logger.info(f"Processing file: {file_id}")
        
        # ファイル処理を実行
        result = file_sorter.process_file(file_id)
        
        if result in ['PROCESSED', 'SKIPPED']:
            logger.info(f"File processed successfully ({result}): {file_id}")
            return 'OK', 200
        else:
            logger.error(f"File processing failed: {file_id}")
            return 'Internal Server Error', 500
            
    except Exception as e:
        logger.error(f"Error handling Pub/Sub message: {e}", exc_info=True)
        return 'Internal Server Error', 500


@app.route('/health', methods=['GET'])
def health_check():
    """ヘルスチェックエンドポイント"""
    return 'OK', 200


@app.route('/test', methods=['POST'])
def test_endpoint():
    """テスト用エンドポイント（手動トリガー）"""
    try:
        # モジュール初期化
        init_modules()
        
        data = request.get_json()
        file_id = data.get('file_id')
        
        if not file_id:
            return {'error': 'file_id is required'}, 400
        
        logger.info(f"Test processing file: {file_id}")
        result = file_sorter.process_file(file_id)
        
        if result in ['PROCESSED', 'SKIPPED']:
            return {'status': 'success', 'result': result, 'file_id': file_id}, 200
        else:
            return {'status': 'failed', 'file_id': file_id}, 500
            
    except Exception as e:
        logger.error(f"Test endpoint error: {e}", exc_info=True)
        return {'error': str(e)}, 500


if __name__ == '__main__':
    port = int(os.environ.get('PORT', 8080))
    app.run(host='0.0.0.0', port=port, debug=True)
