"""
FileSorterモジュール
GASのFileSorter.gsの機能をPythonに移植
"""
import logging
from typing import Dict, Optional, List
from datetime import datetime
from modules.ai_router import AIRouter
from modules.pdf_processor import PDFProcessor
from modules.drive_client import DriveClient
from modules.photos_client import PhotosClient
from modules.calendar_client import CalendarClient
from modules.tasks_client import TasksClient
from modules.grade_manager import GradeManager
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
        photos_client: Optional[PhotosClient] = None,
        calendar_client: Optional[CalendarClient] = None,
        tasks_client: Optional[TasksClient] = None
    ):
        """初期化"""
        self.ai_router = ai_router
        self.pdf_processor = pdf_processor
        self.drive_client = drive_client
        self.photos_client = photos_client
        self.calendar_client = calendar_client
        self.tasks_client = tasks_client
        self.grade_manager = GradeManager()
    
    def process_file(self, file_id: str) -> str:
        """ファイルを処理"""
        try:
            # ファイル情報を取得
            file_info = self.drive_client.get_file(file_id)
            if not file_info:
                logger.error(f"ファイル情報取得失敗: {file_id}")
                return 'ERROR'
            
            file_name = file_info['name']
            mime_type = file_info['mimeType']
            
            logger.info(f"処理開始: {file_name}")
            print(f"[DEBUG] Processing started for: {file_name} ({file_id})", flush=True)
            
            # 対応ファイル形式をチェック
            if mime_type not in SUPPORTED_MIME_TYPES:
                logger.info(f"非対応のファイル形式のためスキップ: {mime_type}")
                return 'SKIPPED'
            
            # ファイルをダウンロード
            print(f"[DEBUG] Downloading file...", flush=True)
            file_bytes = self.drive_client.download_file(file_id)
            if not file_bytes:
                logger.error(f"ファイルダウンロード失敗: {file_id}")
                return 'ERROR'
            print(f"[DEBUG] Download complete. Size: {len(file_bytes)} bytes", flush=True)
            
            # PDFの場合は画像に変換
            if self.pdf_processor.is_pdf(mime_type):
                print(f"[DEBUG] Converting PDF to image...", flush=True)
                images = self.pdf_processor.convert_pdf_to_images(file_bytes)
                if not images:
                    logger.error("PDF変換失敗")
                    return 'ERROR'
                # 最初のページを使用
                image_data = images[0]
                print(f"[DEBUG] PDF conversion complete.", flush=True)
            else:
                image_data = file_bytes
                print(f"[DEBUG] Using original image.", flush=True)
            
            # Geminiで解析
            try:
                analysis_result = self._analyze_document(image_data, file_name)
                if not analysis_result:
                    logger.error("Gemini解析失敗")
                    return 'ERROR'
            except Exception as e:
                logger.error(f"Gemini解析致命的エラー: {e}")
                return 'ERROR'
            
            logger.info(f"解析結果: {analysis_result}")

            # ----------------------------------------------------
            # 子供の特定とフォルダ解決ロジック 
            # ----------------------------------------------------
            category = analysis_result.get('category', '')
            
            if category == '40_子供・教育':
                # 1. 年度計算
                date_str = analysis_result.get('date', '')
                fiscal_year = self.grade_manager.calculate_fiscal_year(date_str)
                analysis_result['fiscal_year'] = fiscal_year # 後続処理のために保存

                # 2. 子供特定
                child_name = analysis_result.get('child_name')
                target_children = []

                if child_name:
                    # 明示的な名前がある場合
                    target_children = [child_name]
                else:
                    # 名前がない場合、学年/クラスから推測
                    grade_class_text = analysis_result.get('target_grade_class', '')
                    if grade_class_text:
                        target_children = self.grade_manager.identify_children(grade_class_text, fiscal_year)
                
                # 特定できた場合、結果に保存（上書き）
                if target_children:
                    # 複数人の場合でも最初の1人を代表としてchild_nameに入れておく（既存ロジック互換性のため）
                    # ただし、フォルダ解決には target_children 全体を使う
                    analysis_result['target_children'] = target_children
                    if not child_name:
                        analysis_result['child_name'] = target_children[0]
                
                # 3. フォルダ名の決定 (共有フォルダ対応)
                # target_children が空の場合はデフォルト処理へ
                folder_name, label, emoji = self.grade_manager.resolve_folder_name(target_children)
                
                if folder_name:
                    analysis_result['resolved_folder_name'] = folder_name
                    analysis_result['resolved_label'] = label
                    analysis_result['resolved_emoji'] = emoji
            
            # ----------------------------------------------------

            # 移動先フォルダを決定
            destination_folder_id = self._get_destination_folder(analysis_result)
            if not destination_folder_id:
                logger.error("移動先フォルダ決定失敗")
                return 'ERROR'
            
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
                return 'ERROR'
            print(f"[DEBUG] Move complete.", flush=True)
            
            logger.info(f"処理完了: {file_name} → {new_file_name}")
            
            # 汎用処理：カテゴリに基づく追加アクション
            sub_category = analysis_result.get('sub_category', '')
            
            # Google Photos アップロード判定
            should_upload_to_photos = (
                category == '50_写真・その他' or
                (category == '40_子供・教育' and sub_category == '03_記録・作品・成績')
            )
            
            if self.photos_client and should_upload_to_photos:
                self._upload_to_photos(image_data, analysis_result)
                
            # Calendar / Tasks 登録判定 (40_子供・教育の場合)
            if category == '40_子供・教育':
                self._register_calendar_and_tasks(image_data, new_file_name, file_id, analysis_result)
            
            return 'PROCESSED'
            
        except Exception as e:
            logger.error(f"ファイル処理エラー: {e}")
            return 'ERROR'
    
    def _analyze_document(
        self,
        image_data: bytes,
        file_name: str
    ) -> Optional[Dict]:
        """ドキュメントを解析"""
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
  "category": "カテゴリ名",
  "child_name": "お子様の名前（名寄せ後の正規名。複数または不明時は空文字）",
  "target_grade_class": "対象となる学年やクラス名（例：小2、くるみ組、1年生）。固有名詞がない場合に抽出",
  "sub_category": "サブカテゴリ（categoryが40_子供・教育の場合のみ）",
  "is_photo": false,
  "date": "YYYYMMDD形式の日付",
  "summary": "要約（15文字以内、ファイル名に使用）",
  "confidence_score": 0.0
}}

## カテゴリ一覧
- 10_マネー・税務
- 20_プロジェクト・資産
- 30_ライフ・行政
- 40_子供・教育
- 50_写真・その他
- 90_ライブラリ

## サブカテゴリ（40_子供・教育の場合のみ使用）
- 01_お便り・スケジュール
- 02_提出・手続き・重要
- 03_記録・作品・成績

## 判断基準
- is_photoがtrueの場合は、categoryを「50_写真・その他」にしてください
- 日付が不明な場合は本日の日付を使用してください
- confidence_scoreは0.0〜1.0の範囲で、解析結果の信頼度を示してください
- 学年やクラス名（「小2」「くるみ組」など）が記載されている場合は、target_grade_classに抽出してください

## ファイル名
{file_name}
"""
        return self.ai_router.analyze_document(image_data, prompt)
    
    def _get_destination_folder(self, result: Dict) -> Optional[str]:
        """移動先フォルダIDを取得"""
        category = result.get('category', '')
        
        if result.get('is_photo', False) or category == '50_写真・その他':
            return FOLDER_IDS['PHOTO_OTHER']
        
        if category == '40_子供・教育':
            return self._get_children_edu_folder(result)
        
        return CATEGORY_MAP.get(category, FOLDER_IDS['PHOTO_OTHER'])
    
    def _get_children_edu_folder(self, result: Dict) -> Optional[str]:
        """40_子供・教育用のフォルダ構造を作成"""
        base_folder_id = FOLDER_IDS['CHILDREN_EDU']
        
        # 解決済みのフォルダ名を使用（なければ child_name または デフォルト）
        folder_name = result.get('resolved_folder_name')
        if not folder_name:
             folder_name = result.get('child_name', '共通・学校全般')

        # 子供名フォルダ (または共有グループフォルダ)
        child_folder_id = self.drive_client.get_or_create_folder(
            folder_name,
            base_folder_id
        )
        if not child_folder_id:
            return None
        
        # 年度フォルダ
        # process_fileで計算済みであればそれを使う
        fiscal_year = result.get('fiscal_year')
        if not fiscal_year:
             # フォールバック
             date_str = result.get('date', '')
             fiscal_year = self.grade_manager.calculate_fiscal_year(date_str)

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
    
    def _generate_new_filename(
        self,
        result: Dict,
        original_name: str
    ) -> str:
        """新しいファイル名を生成"""
        date = result.get('date', datetime.now().strftime('%Y%m%d'))
        summary = result.get('summary', 'document')
        
        parts = original_name.split('.')
        extension = parts[-1] if len(parts) > 1 else 'pdf'
        
        return f"{date}_{summary}.{extension}"
    
    def _upload_to_photos(self, image_data: bytes, result: Dict):
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

    def _register_calendar_and_tasks(
        self,
        image_data: bytes,
        file_name: str,
        file_id: str,
        analysis_result: Dict
    ):
        """カレンダーとタスクに登録"""
        if not self.calendar_client and not self.tasks_client:
            return

        logger.info("カレンダー・タスク抽出処理開始...")
        try:
            # Geminiでスケジュール・タスク情報を抽出
            result = self.ai_router.extract_events_and_tasks(image_data, file_name)
            
            if not result:
                logger.warning("カレンダー・タスク情報抽出失敗（null）")
                return

            # ファイルURL
            file_url = f"https://drive.google.com/file/d/{file_id}/view"

            # プレフィックス作成
            # 解決済みの情報は analysis_result に入っている
            # 単一の子供の場合
            target_children = analysis_result.get('target_children', [])
            fiscal_year = analysis_result.get('fiscal_year')
            
            title_prefix = ""
            if target_children and fiscal_year:
                # 複数の子供がいる場合は、それぞれ個別に登録するか、共有ラベルにするか
                # ここでは共有グループのラベルがあればそれを使い、なければ列挙する
                if analysis_result.get('resolved_emoji'):
                    # グループ絵文字がある場合 (例: [🐿️])
                    title_prefix = f"[{analysis_result['resolved_emoji']}]"
                else:
                    # 個別の場合、最初の子供の学年表記を取得
                    # カレンダー上はスペース節約のため、1人ならその子の学年だけ、複数なら絵文字または名前
                    child_name = target_children[0]
                    grade = self.grade_manager.get_child_grade(child_name, fiscal_year)
                    label, emoji = self.grade_manager.get_grade_info(grade)
                    
                    # ラベル優先順位: Label > Emoji > Name
                    # 保育園も「ぽぷら組」などのテキスト表記を優先（絵文字が化ける可能性があるため）
                    if label: 
                        title_prefix = f"[{label}]"
                    elif emoji:
                        title_prefix = f"[{emoji}]"
                    else:
                        title_prefix = f"[{child_name}]"
            
            elif analysis_result.get('child_name'):
                title_prefix = f"[{analysis_result['child_name']}]"

            # イベント登録
            if self.calendar_client and result.get('events'):
                for event in result['events']:
                    if title_prefix:
                        event['title'] = f"{title_prefix} {event.get('title', '')}"
                    
                    link = self.calendar_client.create_event(event, f"📎 元のお便り: {file_url}")
                    if link:
                        logger.info(f"イベント作成成功: {event.get('title')}")

            # タスク登録
            if self.tasks_client and result.get('tasks'):
                for task in result['tasks']:
                    if title_prefix:
                        task['title'] = f"{title_prefix} {task.get('title', '')}"

                    task_id = self.tasks_client.create_task(task, f"📎 元のお便り: {file_url}")
                    if task_id:
                        logger.info(f"タスク作成成功: {task.get('title')}")

        except Exception as e:
            logger.error(f"カレンダー・タスク登録プロセスエラー: {e}")
