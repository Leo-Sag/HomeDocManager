import os
import json
import sys
import requests

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
PROJECT_ROOT = os.path.abspath(os.path.join(SCRIPT_DIR, ".."))

RICH_MENU_JSON = os.path.join(PROJECT_ROOT, "resources", "linebot", "richmenu.json")
RICH_MENU_IMAGE = os.path.join(PROJECT_ROOT, "resources", "linebot", "richmenu.png")

ACCESS_TOKEN = os.environ.get("LINE_CHANNEL_ACCESS_TOKEN")

if not ACCESS_TOKEN:
    print("Error: LINE_CHANNEL_ACCESS_TOKEN environment variable is not set.", file=sys.stderr)
    print("Set it before running, e.g.:", file=sys.stderr)
    print("  export LINE_CHANNEL_ACCESS_TOKEN=\"$(gcloud secrets versions access latest --secret=LINE_CHANNEL_ACCESS_TOKEN)\"", file=sys.stderr)
    sys.exit(1)

HEADERS = {
    "Authorization": f"Bearer {ACCESS_TOKEN}",
    "Content-Type": "application/json"
}

def setup_rich_menu():
    print("--- Starting Rich Menu Setup ---")

    list_res = requests.get("https://api.line.me/v2/bot/richmenu/list", headers={"Authorization": f"Bearer {ACCESS_TOKEN}"})
    if list_res.status_code != 200:
        print(f"Auth Error: {list_res.status_code} - {list_res.text}")
        return

    existing_menus = list_res.json().get("richmenus", [])
    print(f"Found {len(existing_menus)} existing rich menus.")

    requests.delete("https://api.line.me/v2/bot/user/all/richmenu", headers={"Authorization": f"Bearer {ACCESS_TOKEN}"})
    print("Unset current default rich menu.")

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
