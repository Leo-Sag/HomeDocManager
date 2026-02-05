import requests
import json
import os

# 設定
ACCESS_TOKEN = "S6McakayCvb6EIre8SQqSo96kVi8rrAnlPRUiX4QH2brWOhcDIDfFFs3Eldrdvzxy5ySpBklPh/xuF4spNLlkgsOYGTc/TfAu0jB6J1ZCFH5y85uCuSvE1yVr/sZ8yXrbFOR1/vHz6tRjhrmFrHJEQdB04t89/1O/w1cDnyilFU="
RICH_MENU_JSON = r"k:\.gemini\HomeDocManager\cloud-run-go\resources\linebot\richmenu.json"
RICH_MENU_IMAGE = r"k:\.gemini\HomeDocManager\LINE-bot\LINEbot_richmenu.png"

HEADERS = {
    "Authorization": f"Bearer {ACCESS_TOKEN}",
    "Content-Type": "application/json"
}

def setup_rich_menu():
    print("--- Starting Rich Menu Setup ---")
    
    # 0. トークンの有効性確認 (現在のリストを取得)
    list_res = requests.get("https://api.line.me/v2/bot/richmenu/list", headers={"Authorization": f"Bearer {ACCESS_TOKEN}"})
    if list_res.status_code != 200:
        print(f"Auth Error: {list_res.status_code} - {list_res.text}")
        return
    
    existing_menus = list_res.json().get("richmenus", [])
    print(f"Found {len(existing_menus)} existing rich menus.")

    # 1. 既存のデフォルト設定を解除
    requests.delete("https://api.line.me/v2/bot/user/all/richmenu", headers={"Authorization": f"Bearer {ACCESS_TOKEN}"})
    print("Unset current default rich menu.")

    # 2. リッチメニューの作成
    with open(RICH_MENU_JSON, 'r', encoding='utf-8') as f:
        rich_menu_data = json.load(f)
    
    print(f"Creating rich menu...")
    res = requests.post(
        "https://api.line.me/v2/bot/richmenu",
        headers=HEADERS,
        data=json.dumps(rich_menu_data)
    )
    
    if res.status_code != 200:
        print(f"Error creating rich menu: Status {res.status_code}")
        print(f"Response: {res.text}")
        return
    
    rich_menu_id = res.json().get("richMenuId")
    print(f"Successfully created rich menu ID: {rich_menu_id}")

    # 3. 画像のアップロード (api-data.line.meを使用)
    with open(RICH_MENU_IMAGE, 'rb') as f:
        img_res = requests.post(
            f"https://api-data.line.me/v2/bot/richmenu/{rich_menu_id}/content",
            headers={
                "Authorization": f"Bearer {ACCESS_TOKEN}",
                "Content-Type": "image/png"
            },
            data=f
        )
    
    if img_res.status_code != 200:
        print(f"Error uploading image: {img_res.status_code} - {img_res.text}")
        return
    print("Successfully uploaded rich menu image.")

    # 4. デフォルトリッチメニューに設定
    def_res = requests.post(
        f"https://api.line.me/v2/bot/user/all/richmenu/{rich_menu_id}",
        headers={"Authorization": f"Bearer {ACCESS_TOKEN}"}
    )
    
    if def_res.status_code != 200:
        print(f"Error setting default: {def_res.status_code} - {def_res.text}")
        return
    print("--- Successfully set as default rich menu! ---")

if __name__ == "__main__":
    setup_rich_menu()
