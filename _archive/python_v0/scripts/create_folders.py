from modules.drive_client import DriveClient
import os
import sys

# Add project root to path
sys.path.append(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

def create_folders():
    client = DriveClient()
    
    # Create under root for now, or user can move them later
    print("Creating folders under 'root'...")
    
    output_folder_id = client.create_folder("PDF_Conversion_Output", 'root')
    print(f"Created PDF_Conversion_Output: {output_folder_id}")
    
    archive_folder_id = client.create_folder("PDF_Conversion_Archive", 'root')
    print(f"Created PDF_Conversion_Archive: {archive_folder_id}")

if __name__ == "__main__":
    create_folders()
