"""
Google Calendar API Client
"""
import logging
from datetime import datetime, timedelta
from typing import Dict, Optional
from googleapiclient.discovery import build
from modules.auth_utils import get_oauth_credentials

from config.settings import CALENDAR_ID

logger = logging.getLogger(__name__)


class CalendarClient:
    """Client for Google Calendar API"""
    
    def __init__(self):
        self.credentials = get_oauth_credentials()
        self.service = build('calendar', 'v3', credentials=self.credentials)
        
    def create_event(self, event_data: Dict, description_append: str = "") -> Optional[str]:
        """
        Create an event in the specified calendar.
        
        Args:
            event_data (Dict): Event data containing title, date, start_time, end_time, location, description.
            description_append (str): Text to append to the description (e.g. file URL).
            
        Returns:
            Optional[str]: HTML link to the created event.
        """
        try:
            summary = event_data.get('title', 'No Title')
            description = f"{event_data.get('description', '')}\n\n{description_append}".strip()
            location = event_data.get('location', '')
            date_str = event_data.get('date')
            
            if not date_str:
                logger.warning("Event date is missing")
                return None

            event_body = {
                'summary': summary,
                'description': description,
                'location': location,
            }

            start_time_str = event_data.get('start_time')
            end_time_str = event_data.get('end_time')

            if start_time_str:
                # Timed event
                start_dt = self._parse_datetime(date_str, start_time_str)
                
                if end_time_str:
                    end_dt = self._parse_datetime(date_str, end_time_str)
                else:
                    # Default duration: 1 hour
                    end_dt = start_dt + timedelta(hours=1)
                
                event_body['start'] = {'dateTime': start_dt.isoformat(), 'timeZone': 'Asia/Tokyo'}
                event_body['end'] = {'dateTime': end_dt.isoformat(), 'timeZone': 'Asia/Tokyo'}
                logger.info(f"Creating timed event: {summary} ({start_dt})")
            else:
                # All-day event
                event_body['start'] = {'date': date_str}
                event_body['end'] = {'date': date_str} # Google Calendar requires end date for all-day events (exclusive? checking usually +1 day)
                # Actually for all-day, end is exclusive, so we should probably add 1 day if it's single day.
                # But let's check GAS implementation. GAS: createAllDayEvent(title, date). 
                # API v3: end.date is exclusive. If start=2024-01-01, end=2024-01-02 means 1 day.
                # If we set start=end, it might be invalid.
                
                # Let's safely add 1 day for the end date string
                start_date_obj = datetime.strptime(date_str, '%Y-%m-%d')
                end_date_obj = start_date_obj + timedelta(days=1)
                event_body['end']['date'] = end_date_obj.strftime('%Y-%m-%d')
                
                logger.info(f"Creating all-day event: {summary} ({date_str})")

            created_event = self.service.events().insert(calendarId=CALENDAR_ID, body=event_body).execute()
            return created_event.get('htmlLink')

        except Exception as e:
            logger.error(f"Failed to create calendar event: {e}")
            return None

    def _parse_datetime(self, date_str: str, time_str: str) -> datetime:
        """Parse date and time strings into datetime object."""
        # Clean up time string (sometimes might be '10:00')
        return datetime.strptime(f"{date_str} {time_str}", '%Y-%m-%d %H:%M')
