"""
AIルーターモジュール
Gemini 3 Flash優先、信頼度スコアに基づくProへのエスカレーション
"""
import json
import logging
from typing import Dict, Optional
from google.cloud import secretmanager
import google.generativeai as genai
from config.settings import (
    GEMINI_MODELS, 
    AI_ROUTER_CONFIG,
    GCP_PROJECT_ID,
    SECRET_GEMINI_API_KEY
)

logger = logging.getLogger(__name__)


class AIRouter:
    """Gemini 3 Flash/Proを使い分けるAIルーター"""
    
    def __init__(self):
        """初期化"""
        self.api_key = self._get_api_key()
        genai.configure(api_key=self.api_key)
        
    def _get_api_key(self) -> str:
        """Secret ManagerからGemini APIキーを取得"""
        try:
            client = secretmanager.SecretManagerServiceClient()
            name = f"projects/{GCP_PROJECT_ID}/secrets/{SECRET_GEMINI_API_KEY}/versions/latest"
            response = client.access_secret_version(request={"name": name})
            return response.payload.data.decode('UTF-8').strip()
        except Exception as e:
            logger.error(f"Secret Manager APIキー取得エラー: {e}")
            # フォールバック: 環境変数から取得
            import os
            api_key = os.getenv('GEMINI_API_KEY')
            if not api_key:
                raise ValueError("Gemini API keyが見つかりません")
            return api_key.strip()
    
    def analyze_document(
        self, 
        image_data: bytes, 
        prompt: str,
        use_flash_first: bool = True
    ) -> Optional[Dict]:
        """
        ドキュメントを解析（AIルーターパターン）
        
        Args:
            image_data: 画像バイナリデータ
            prompt: プロンプト
            use_flash_first: Flash優先フラグ
            
        Returns:
            解析結果（JSON）
        """
        if use_flash_first:
            # 第一段階: Gemini 3 Flash
            logger.info("Gemini 3 Flashで解析開始")
            flash_result = self._call_gemini(
                GEMINI_MODELS['FLASH'],
                image_data,
                prompt
            )
            
            if flash_result and self._is_confident(flash_result):
                logger.info(f"Flash解析成功（信頼度: {flash_result.get('confidence_score', 'N/A')}）")
                return flash_result
            
            # 第二段階: Gemini 3 Proへエスカレーション
            if AI_ROUTER_CONFIG['ENABLE_PRO_ESCALATION']:
                logger.warning(
                    f"信頼度が低いためProにエスカレーション "
                    f"(score: {flash_result.get('confidence_score', 0.0) if flash_result else 'N/A'})"
                )
                return self._call_gemini(
                    GEMINI_MODELS['PRO'],
                    image_data,
                    prompt
                )
            
            return flash_result
        else:
            # Pro直接呼び出し
            return self._call_gemini(
                GEMINI_MODELS['PRO'],
                image_data,
                prompt
            )
    
    def _call_gemini(
        self, 
        model_name: str, 
        image_data: bytes, 
        prompt: str
    ) -> Optional[Dict]:
        """Gemini APIを呼び出し"""
        try:
            # デバッグ: APIキーの確認（先頭/末尾のみ表示）
            if self.api_key:
                masked_key = f"{self.api_key[:4]}...{self.api_key[-4:]}"
                logger.info(f"Using API Key: {masked_key} (Length: {len(self.api_key)})")
            else:
                logger.error("API Key is empty/None")

            model = genai.GenerativeModel(
                model_name,
                generation_config={
                    "response_mime_type": "application/json"
                }
            )
            
            # 画像とプロンプトを送信
            import base64
            image_b64 = base64.b64encode(image_data).decode('utf-8')
            
            response = model.generate_content([
                {
                    "mime_type": "image/jpeg",
                    "data": image_b64
                },
                prompt
            ])
            
            # JSON形式でパース
            result = json.loads(response.text)
            return result
            
        except Exception as e:
            # repr()を使ってエンコーディング問題を回避
            logger.error(f"Gemini API呼び出しエラー ({model_name}): {repr(e)}")
            import traceback
            logger.error(f"Traceback: {traceback.format_exc()}")
            return None
    
    def _is_confident(self, result: Dict) -> bool:
        """信頼度スコアをチェック"""
        if not result:
            return False
        confidence = result.get('confidence_score', 0.0)
        threshold = AI_ROUTER_CONFIG['CONFIDENCE_THRESHOLD']
        return confidence >= threshold

    def extract_events_and_tasks(
        self,
        image_data: bytes,
        file_name: str
    ) -> Optional[Dict]:
        """
        ドキュメントから予定とタスクを抽出
        
        Args:
            image_data: 画像バイナリデータ
            file_name: ファイル名
            
        Returns:
            抽出結果（JSON）
        """
        from datetime import datetime
        today = datetime.now().strftime('%Y-%m-%d')
        
        prompt = f"""
あなたは学校のお便りから予定とタスクを抽出するアシスタントです。
以下の画像を解析し、JSON形式で回答してください。

## 出力形式（必ずこのJSON形式で回答）
{{
  "events": [
    {{
      "title": "イベントタイトル",
      "date": "YYYY-MM-DD",
      "start_time": "HH:MM（不明な場合は null）",
      "end_time": "HH:MM（不明な場合は null）",
      "location": "場所（不明な場合は null）",
      "description": "詳細説明"
    }}
  ],
  "tasks": [
    {{
      "title": "タスクタイトル（例：○○の提出）",
      "due_date": "YYYY-MM-DD",
      "notes": "備考"
    }}
  ]
}}

## 判断基準
- **events**: 日時が確定している行事（運動会、授業参観、保護者会など）
- **tasks**: 期限がある提出物や準備事項（書類提出、持ち物準備など）

## 注意事項
- 過去の日付（{today}より前）のイベント・タスクは除外してください
- 年が明示されていない場合は、{today[:4]}年と仮定してください
- 抽出できる情報がない場合は、eventsとtasksを空配列にしてください

## ファイル名
{file_name}
"""
        # 構造化抽出のためProモデル使用またはFlashでもJSONモード
        # ここでは既存の _call_gemini を使用 (Flash優先)
        return self.analyze_document(image_data, prompt, use_flash_first=True)
