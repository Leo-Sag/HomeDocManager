import os
import zipfile
import datetime

def backup_project(source_dir, output_zip):
    # Exclude patterns
    excludes = {
        '__pycache__',
        '.git',
        '.venv',
        'venv',
        '.env', # Backup env? Maybe risky if shared, but for local backup usually wanted. User didn't ask to exclude secrets.
                # Actually, Cloud Run code doesn't strictly depend on local .env if using Secret Manager.
                # Let's verify if .env exists. From context, .env.example exists.
        'node_modules',
        '*.pyc'
    }

    print(f"Backing up '{source_dir}' to '{output_zip}'...")

    with zipfile.ZipFile(output_zip, 'w', zipfile.ZIP_DEFLATED) as zipf:
        for root, dirs, files in os.walk(source_dir):
            # Modify dirs in-place to skip excluded directories
            dirs[:] = [d for d in dirs if d not in excludes]
            
            for file in files:
                if file in excludes or file.endswith('.pyc'):
                    continue
                
                file_path = os.path.join(root, file)
                arcname = os.path.relpath(file_path, start=source_dir)
                zipf.write(file_path, arcname)
                print(f"Added: {arcname}")

    print("Backup complete!")

if __name__ == "__main__":
    source = r"k:/.gemini/HomeDocManager"
    timestamp = datetime.datetime.now().strftime("%Y%m%d_%H%M%S")
    output = f"k:/.gemini/HomeDocManager_v1.0.0_{timestamp}.zip"
    backup_project(source, output)
