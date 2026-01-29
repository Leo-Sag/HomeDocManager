"""
Google Tasks API Client
"""
import logging
from datetime import datetime
from typing import Dict, Optional
from googleapiclient.discovery import build
from modules.auth_utils import get_oauth_credentials

logger = logging.getLogger(__name__)


class TasksClient:
    """Client for Google Tasks API"""
    
    def __init__(self):
        self.credentials = get_oauth_credentials()
        self.service = build('tasks', 'v1', credentials=self.credentials)
        
    def create_task(self, task_data: Dict, notes_append: str = "") -> Optional[str]:
        """
        Create a task in the default task list.
        
        Args:
            task_data (Dict): Task data containing title, due_date, notes.
            notes_append (str): Text to append to the notes (e.g. file URL).
            
        Returns:
            Optional[str]: ID of the created task.
        """
        try:
            title = task_data.get('title', 'No Title')
            notes = f"{task_data.get('notes', '')}\n\n{notes_append}".strip()
            due_date_str = task_data.get('due_date')
            
            task_body = {
                'title': title,
                'notes': notes,
            }

            if due_date_str:
                # Tasks API requires RFC3339 timestamp string for 'due'
                # Note: 'due' is technically a date-time, but usually treated as date.
                # However, the API expects a timestamp string. 
                # Let's set it to T00:00:00Z of that day to be safe, or simply the string if format allows.
                # Actually, the Tasks API doc says: "Due date of the task (as a RFC 3339 timestamp). Optional. 
                # The due date cannot be earlier than the validation date."
                # We'll set it to 23:59:59 of that day to avoid confusion about "passed due".
                try:
                    due_dt = datetime.strptime(due_date_str, '%Y-%m-%d')
                    # Set to end of day in UTC roughly (or simply construct ISO string)
                    # Ideally, we should handle timezone properly, but Tasks 'due' is often date-only in UI.
                    # Let's use T00:00:00Z for simplicity as it usually maps to that date.
                    task_body['due'] = due_dt.strftime('%Y-%m-%dT00:00:00Z')
                except ValueError:
                    logger.warning(f"Invalid due date format: {due_date_str}")

            logger.info(f"Creating task: {title}")
            created_task = self.service.tasks().insert(tasklist='@default', body=task_body).execute()
            return created_task.get('id')

        except Exception as e:
            logger.error(f"Failed to create task: {e}")
            return None
