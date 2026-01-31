"""
NotebookLM Markdown同期のデバッグスクリプト
Markdownファイルの作成と追記をテスト
"""
import os
import sys

# Add parent directory to path
sys.path.insert(0, os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

from modules.drive_client import DriveClient
from modules.notebooklm_sync import NotebookLMSync
from config.settings import FOLDER_IDS

def test_markdown_sync():
    """Markdown同期のテスト"""
    print("=" * 50)
    print("NotebookLM Markdown同期テスト")
    print("=" * 50)
    
    # 初期化
    print("\n[1] クライアント初期化...")
    drive_client = DriveClient()
    sync = NotebookLMSync(drive_client)
    
    # テストデータ
    test_fiscal_year = 2025  # テスト用年度
    test_file_id = "test_file_id_12345"
    test_file_name = "テスト書類.pdf"
    test_category = "学校"
    test_ocr_text = "これはテスト用のOCRテキストです。\n日付: 2025年1月31日\n内容: テストコンテンツ"
    test_date_str = "20250131"
    
    # ファイル作成/取得テスト
    print(f"\n[2] {test_fiscal_year}年度のMarkdownファイルを取得/作成...")
    doc_id = sync._get_or_create_accumulated_doc(test_fiscal_year)
    
    if not doc_id:
        print("❌ ファイルの取得/作成に失敗しました")
        return False
    
    print(f"✅ ファイルID: {doc_id}")
    
    # ファイル情報を確認
    print("\n[3] ファイル情報を確認...")
    file_info = drive_client.service.files().get(
        fileId=doc_id,
        fields='name, mimeType, webViewLink'
    ).execute()
    
    print(f"   名前: {file_info.get('name')}")
    print(f"   MIMEタイプ: {file_info.get('mimeType')}")
    print(f"   URL: {file_info.get('webViewLink')}")
    
    # 追記テスト
    print("\n[4] テストエントリを追記...")
    formatted_date = sync._format_date(test_date_str)
    entry_text = sync._format_entry(
        formatted_date, 
        test_file_name, 
        test_file_id, 
        test_ocr_text, 
        test_category
    )
    
    success = sync._append_to_doc(doc_id, entry_text)
    
    if success:
        print("✅ 追記成功")
    else:
        print("❌ 追記失敗")
        return False
    
    # 内容を確認
    print("\n[5] ファイル内容を確認...")
    content = drive_client.service.files().get_media(fileId=doc_id).execute()
    if isinstance(content, bytes):
        content = content.decode('utf-8')
    
    print("-" * 40)
    print(content[:1000] if len(content) > 1000 else content)
    print("-" * 40)
    
    print("\n✅ テスト完了!")
    print(f"   確認URL: {file_info.get('webViewLink')}")
    return True


if __name__ == "__main__":
    try:
        success = test_markdown_sync()
        sys.exit(0 if success else 1)
    except Exception as e:
        print(f"❌ エラー: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
