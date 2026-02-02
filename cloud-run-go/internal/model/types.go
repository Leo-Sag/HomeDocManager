package model

import "time"

// AnalysisResult はGeminiによるドキュメント解析結果
type AnalysisResult struct {
	Category         string  `json:"category"`
	ChildName        string  `json:"child_name"`
	TargetAdult      string  `json:"target_adult"`
	TargetGradeClass string  `json:"target_grade_class"`
	SubCategory      string  `json:"sub_category"`
	IsPhoto          bool    `json:"is_photo"`
	Date             string  `json:"date"`
	Summary          string  `json:"summary"`
	ConfidenceScore  float64 `json:"confidence_score"`
	// 内部処理用フィールド
	FiscalYear         int      `json:"-"`
	TargetChildren     []string `json:"-"`
	ResolvedFolderName string   `json:"-"`
	ResolvedLabel      string   `json:"-"`
	ResolvedEmoji      string   `json:"-"`
}

// EventsAndTasks はカレンダー・タスク抽出結果
type EventsAndTasks struct {
	Events []Event `json:"events"`
	Tasks  []Task  `json:"tasks"`
}

// Event はカレンダーイベント
type Event struct {
	Title       string  `json:"title"`
	Date        string  `json:"date"`
	StartTime   *string `json:"start_time"`
	EndTime     *string `json:"end_time"`
	Location    *string `json:"location"`
	Description string  `json:"description"`
}

// Task はタスク
type Task struct {
	Title   string `json:"title"`
	DueDate string `json:"due_date"`
	Notes   string `json:"notes"`
}

// PubSubMessage はPub/Subメッセージ
type PubSubMessage struct {
	Message struct {
		Data        string            `json:"data"`
		MessageID   string            `json:"messageId"`
		Attributes  map[string]string `json:"attributes"`
		PublishTime time.Time         `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// FileData はPub/Subメッセージのデータ
type FileData struct {
	FileID string `json:"file_id"`
}

// FileInfo はGoogle Driveファイル情報
type FileInfo struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	MimeType string   `json:"mimeType"`
	Parents  []string `json:"parents"`
}

// ProcessResult はファイル処理結果
type ProcessResult string

const (
	ProcessResultProcessed ProcessResult = "PROCESSED"
	ProcessResultSkipped   ProcessResult = "SKIPPED"
	ProcessResultError     ProcessResult = "ERROR"
)

// OCRBundle はOCR結果の構造化データ
type OCRBundle struct {
	OCRText         string   `json:"ocr_text"`
	Facts           []string `json:"facts"`
	Summary         string   `json:"summary"`
	ConfidenceScore float64  `json:"confidence_score"`
	Quality         struct {
		Uncertain      bool   `json:"uncertain"`
		NeedsHighModel bool   `json:"needs_high_model"`
		Notes          string `json:"notes"`
	} `json:"quality"`
}
