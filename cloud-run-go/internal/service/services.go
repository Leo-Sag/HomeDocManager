package service

// Services は全サービスをまとめた構造体
type Services struct {
	AIRouter        *AIRouter
	PDFProcessor    *PDFProcessor
	DriveClient     *DriveClient
	PhotosClient    *PhotosClient
	CalendarClient  *CalendarClient
	TasksClient     *TasksClient
	NotebookLMSync  *NotebookLMSync
	GradeManager    *GradeManager
	FileSorter      *FileSorter
	DiscordNotifier *DiscordNotifier
}
