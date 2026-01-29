"""
GradeManagerモジュール
日付に基づく学年計算とクラス判定、子供の特定を行う
"""
import logging
import re
from datetime import datetime
from typing import Dict, List, Optional, Tuple, Any
from config.settings import GRADE_CONFIG, CHILD_ALIASES

logger = logging.getLogger(__name__)

class GradeManager:
    """学年・クラス管理クラス"""
    
    def __init__(self):
        self.config = GRADE_CONFIG
        self.base_fy = self.config['BASE_FISCAL_YEAR']
        self.base_grades = self.config['CHILDREN_BASE_GRADES']
        self.preschool_classes = self.config['PRESCHOOL_CLASSES']
        self.shared_groups = self.config['SHARED_GROUPS']

    def calculate_fiscal_year(self, date_str: str) -> int:
        """
        日付文字列から年度を計算（4月始まり）
        YYYYMMDD形式を想定
        """
        try:
            if not date_str or len(date_str) != 8:
                # 日付不明の場合は現在の年度
                now = datetime.now()
                year = now.year
                if now.month <= 3:
                    return year - 1
                return year

            year = int(date_str[:4])
            month = int(date_str[4:6])
            
            # 1〜3月は前年度扱い
            if 1 <= month <= 3:
                return year - 1
            return year
        except Exception as e:
            logger.warning(f"年度計算エラー ({date_str}): {e}")
            # エラー時は現在の年度
            now = datetime.now()
            year = now.year
            if now.month <= 3:
                return year - 1
            return year

    def get_child_grade(self, child_name: str, fiscal_year: int) -> int:
        """指定年度における子供の学年コードを取得"""
        # 名前を正規化
        normalized_name = self._normalize_child_name(child_name)
        if not normalized_name or normalized_name not in self.base_grades:
            return -99 # 不明

        base_grade = self.base_grades[normalized_name]
        year_diff = fiscal_year - self.base_fy
        
        current_grade = base_grade + year_diff
        return current_grade

    def get_grade_info(self, grade_value: int) -> Tuple[str, str]:
        """
        学年コードから表記と絵文字を取得
        Returns: (label, emoji)
        例: ("小2", "🏫"), ("ぽぷら組", "🌳")
        """
        # 保育園 (-6 ~ -1)
        if grade_value in self.preschool_classes:
            info = self.preschool_classes[grade_value]
            return info['name'], info['emoji']
        
        # 小学校 (1 ~ 6)
        if 1 <= grade_value <= 6:
            return f"小{grade_value}", "🏫"
        
        # 中学校 (7 ~ 9)
        if 7 <= grade_value <= 9:
            return f"中{grade_value - 6}", "🏫"
        
        # 高校 (10 ~ 12)
        if 10 <= grade_value <= 12:
            return f"高{grade_value - 9}", "🏫"
            
        return "", ""

    def identify_children(self, text: str, fiscal_year: int) -> List[str]:
        """
        テキスト（名前、学年、クラス名）から該当する子供のリストを取得
        """
        # 1. まず名前が明示されているかチェック
        found_children = set()
        for child_key, aliases in CHILD_ALIASES.items():
            for alias in aliases:
                if alias in text:
                    found_children.add(child_key)
        
        if found_children:
            return list(found_children)

        # 2. クラス名・学年からの推測
        # 保育園クラス名チェック
        for grade, info in self.preschool_classes.items():
            if info['name'] in text:
                return self._get_children_by_grade(grade, fiscal_year)

        # 学年表記チェック (正規表現)
        # 小学校
        match = re.search(r'小([1-6１-６])', text) or re.search(r'小学([1-6１-６])年生?', text)
        if match:
            grade = int(match.group(1).translate(str.maketrans('１２３４５６', '123456')))
            return self._get_children_by_grade(grade, fiscal_year)
            
        # 中学校
        match = re.search(r'中([1-3１-３])', text) or re.search(r'中学([1-3１-３])年生?', text)
        if match:
            grade = int(match.group(1).translate(str.maketrans('１２３', '123'))) + 6
            return self._get_children_by_grade(grade, fiscal_year)
            
        # 高校
        match = re.search(r'高([1-3１-３])', text) or re.search(r'高校([1-3１-３])年生?', text)
        if match:
            grade = int(match.group(1).translate(str.maketrans('１２３', '123'))) + 9
            return self._get_children_by_grade(grade, fiscal_year)

        return []

    def _get_children_by_grade(self, target_grade: int, fiscal_year: int) -> List[str]:
        """指定年度に指定学年である子供を探す"""
        matching_children = []
        for child_name in self.base_grades.keys():
            if self.get_child_grade(child_name, fiscal_year) == target_grade:
                matching_children.append(child_name)
        return matching_children

    def _normalize_child_name(self, name: str) -> Optional[str]:
        """名前の正規化"""
        for normalized, aliases in CHILD_ALIASES.items():
            if name in aliases or name == normalized:
                return normalized
        return None

    def resolve_folder_name(self, children: List[str]) -> Tuple[str, str, str]:
        """
        子供リストから格納先フォルダ名とラベル情報を決定
        Returns: (folder_name, display_label, emoji)
        """
        if not children:
            return None, "", ""

        # 単独の場合
        if len(children) == 1:
            child = children[0]
            # 学年情報を取得（現在の年度を基準とするか、引数でもらうか...ここでは簡易的に呼び出し元で処理してもらう前提で、子供の名前だけ返す手もあるが、要件に合わせてグループ解決を行う）
            return child, child, "" # 絵文字は呼び出し元で年度解決後に付与

        # 複数の場合、共有グループ定義をチェック
        # 子供リストをセットで比較
        children_set = set(children)
        for group_name, info in self.shared_groups.items():
            group_children = set(info['children'])
            # グループの子供が全て含まれているか、あるいはグループの子供のサブセットか
            # 今回の場合、"Kurumi"クラスの書類 = 3人全員 とみなすのが自然
            if children_set == group_children:
                return info['folder_name'], group_name, info['label']
            
            # 部分一致の場合（例：遥香とアンナだけの書類）も共有フォルダに入れる？
            # 一旦、グループのメンバーが複数含まれていれば共有フォルダとするロジックも検討できるが、
            # シンプルに「グループ定義に完全一致」または「代表者フォルダ」とする
        
        # 定義されていない組み合わせの場合は、最初の子供のフォルダにするか、連名にする
        # 今回は連名フォルダを動的に作るよりは、最初の子供をPrimaryとする（または呼び出し元で処理）
        return children[0], children[0], ""
