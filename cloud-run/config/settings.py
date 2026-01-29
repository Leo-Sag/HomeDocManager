"""
設定ファイル - Config.gsの内容を移植
"""
import os
from typing import Dict, List

# GCPプロジェクト設定
GCP_PROJECT_ID = os.getenv('GCP_PROJECT_ID', 'your-project-id')
GCP_REGION = os.getenv('GCP_REGION', 'asia-northeast1')

# Secret Manager設定
SECRET_PHOTOS_REFRESH_TOKEN = 'PHOTOS_REFRESH_TOKEN'
SECRET_GEMINI_API_KEY = 'GEMINI_API_KEY'

# Gemini APIモデル設定
# Gemini 3 は高精度な解析が可能。Flash優先、信頼度低下時にProへエスカレーション
GEMINI_MODELS = {
    'FLASH': 'gemini-3-flash-preview',
    'PRO': 'gemini-3-pro-preview'
}

# AIルーター設定
AI_ROUTER_CONFIG = {
    'CONFIDENCE_THRESHOLD': 0.8,  # この値未満でProにエスカレーション
    'MAX_FLASH_RETRIES': 2,
    'ENABLE_PRO_ESCALATION': True
}

# Google Driveフォルダ設定（Config.gsから移植）
FOLDER_IDS = {
    'SOURCE': '1T_XJURJbSsSiarr2Y-ofH0lCpSn9Dmak',
    'MONEY_TAX': '1rUnmoPoJoD-UwLn0PQW7-FtBfg9FlUTi',
    'PROJECT_ASSET': '1xBNSHmmnpuQpz0pvXxg_VlUAy0Zk4SOG',
    'LIFE_ADMIN': '1keZdfSSrmpPqPWhC22Fg2A5GmaCfg3Xg',
    'CHILDREN_EDU': '14TyZrKoXRSSP6kxpytxvap4poKmDn4qs',
    'PHOTO_OTHER': '1euBhhNI0Ny13tXs1JVrcO0KLKHySFnEy',
    'LIBRARY': '1MxppChMYZOJOyY2s-w6CsVam3P5_vccv',
    'NOTEBOOKLM_SYNC': '1AVRbK5Zy8IVC3XYtSQ7ZwNGMIB3ToaBu',
    'ARCHIVE': '14iqjkHeBVMz47sNzPFkxrp5syr2tIOeO'
}

# カテゴリマッピング
CATEGORY_MAP: Dict[str, str] = {
    '10_マネー・税務': FOLDER_IDS['MONEY_TAX'],
    '20_プロジェクト・資産': FOLDER_IDS['PROJECT_ASSET'],
    '30_ライフ・行政': FOLDER_IDS['LIFE_ADMIN'],
    '40_子供・教育': FOLDER_IDS['CHILDREN_EDU'],
    '50_写真・その他': FOLDER_IDS['PHOTO_OTHER'],
    '90_ライブラリ': FOLDER_IDS['LIBRARY'],
    '99_転送済みアーカイブ': FOLDER_IDS['ARCHIVE']
}

# 子供の名寄せルール
CHILD_ALIASES: Dict[str, List[str]] = {
    '明日香': ['明日香', 'あすか', 'アスカ', 'Asuka'],
    '遥香': ['遥香', 'はるか', 'ハルカ', 'Haruka'],
    '文香': ['文香', 'ふみか', 'フミカ', 'Fumika'],
    'ビクトル': ['ビクトル', 'Victor', 'Viktor'],
    'ミハイル': ['ミハイル', 'Mikhail', 'Mihail'],
    'アンナ': ['アンナ', 'Anna']
}

# 大人の名寄せルール
# カレンダー/タスクのラベルには正規名をそのまま使用
ADULT_ALIASES: Dict[str, List[str]] = {
    '千世己': ['千世己', 'Chiseki', 'ちせき', 'チセキ'],
    'まどか': ['まどか', 'Madoka', 'マドカ'],
    '怜央奈': ['怜央奈', 'Leo', 'Reona', 'れおな', 'レオナ'],
    '今日子': ['今日子', 'Kyoko', 'きょうこ', '綿谷', 'Wataya'],
    'えりか': ['えりか', 'Erika', 'エリカ', 'Эрика']
}

# 年度サブフォルダを作成するカテゴリ
CATEGORIES_WITH_YEAR_SUBFOLDER: List[str] = [
    '10_マネー・税務',
    '30_ライフ・行政',
    '40_子供・教育'
]

# NotebookLM同期対象カテゴリ
NOTEBOOKLM_SYNC_CATEGORIES: List[str] = [
    '10_マネー・税務',
    '20_プロジェクト・資産',
    '30_ライフ・行政',
    '40_子供・教育',
    '90_ライブラリ'
]

# NotebookLMドキュメントのオーナー（サービスアカウントの容量制限回避用）
NOTEBOOKLM_OWNER_EMAIL: str = 'leo.courageous.lion@gmail.com'

# 子供の卒業設定
# 高校3年（学年コード12）を超えると大人として扱う
CHILD_GRADUATION_GRADE: int = 12

# 大人用カテゴリ（卒業後の子供の書類はこれらに振り分け）
ADULT_CATEGORIES: List[str] = [
    '10_マネー・税務',
    '30_ライフ・行政'
]

# 学年・クラス設定
# 基準年度: 2024年度 (2024/4/1 - 2025/3/31)
# 学年コード: 小1=1, 中1=7, 高1=10, 年長=-1, 年中=-2, 年少=-3
GRADE_CONFIG = {
    'BASE_FISCAL_YEAR': 2024,
    'CHILDREN_BASE_GRADES': {
        'ビクトル': 2,    # 小2
        '明日香': -1,   # 年長 (ぽぷら)
        '遥香': -3,     # 年少 (くるみ)
        'アンナ': -3,   # 年少 (くるみ)
        'ミハイル': -3, # 年少 (くるみ)
        '文香': -5,     # 1歳児 (りんご)
    },
    'PRESCHOOL_CLASSES': {
        -1: {'name': 'ぽぷら組', 'emoji': '🌳'},
        -2: {'name': 'いちょう組', 'emoji': '🍂'},
        -3: {'name': 'くるみ組', 'emoji': '🐿️'},
        -4: {'name': 'たんぽぽ組', 'emoji': '🌼'},
        -5: {'name': 'りんご組', 'emoji': '🍎'},
        -6: {'name': 'さくらんぼ組', 'emoji': '🍒'},
    },
    'SHARED_GROUPS': {
        'くるみ組': {
            'children': ['遥香', 'アンナ', 'ミハイル'],
            'folder_name': 'Haruka-Anna-Mischa',
            'label': '🐿️'
        },
        'いちょう組': {
            'children': ['遥香', 'アンナ', 'ミハイル'],
            'folder_name': 'Haruka-Anna-Mischa',
            'label': '🍂'
        },
        'ぽぷら組': {
            'children': ['遥香', 'アンナ', 'ミハイル'],
            'folder_name': 'Haruka-Anna-Mischa',
            'label': '🌳'
        }
    }
}

# サブカテゴリ
SUB_CATEGORIES = [
    '01_お便り・スケジュール',
    '02_提出・手続き・重要',
    '03_記録・作品・成績'
]


# CalendarSync Configuration
TARGET_SUBFOLDER_NAMES = [
    '01_お便り・スケジュール',
    '02_提出・手続き・重要'
]

CALENDAR_ID = os.getenv('CALENDAR_ID', '639243bb722810f6fbe8f95b9dc57adf65677a53810d7fcdc76eef0fc4845792@group.calendar.google.com')

# API設定
API_CONFIG = {
    'TIMEOUT_MS': 30000,
    'MAX_RETRIES': 3,
    'RETRY_DELAY_MS': 1000
}

# 対応ファイル形式
SUPPORTED_MIME_TYPES = [
    'application/pdf',
    'image/jpeg',
    'image/png',
    'image/gif',
    'image/bmp'
]
