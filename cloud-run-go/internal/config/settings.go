package config

import (
	"os"
)

// GCP設定
var (
	GCPProjectID = GetEnv("GCP_PROJECT_ID", "your-project-id")
	GCPRegion    = GetEnv("GCP_REGION", "asia-northeast1")
)

// Secret Manager設定
const (
	SecretPhotosRefreshToken = "PHOTOS_REFRESH_TOKEN"
	SecretGeminiAPIKey       = "GEMINI_API_KEY"
)

// Gemini APIモデル設定
type GeminiModels struct {
	Flash string
	Pro   string
}

var GeminiModelsConfig = GeminiModels{
	Flash: "gemini-3-flash-preview",
	Pro:   "gemini-3-pro-preview",
}

// AIルーター設定
type AIRouterConfig struct {
	ConfidenceThreshold float64
	MaxFlashRetries     int
	EnableProEscalation bool
}

var AIRouter = AIRouterConfig{
	ConfidenceThreshold: 0.8,
	MaxFlashRetries:     2,
	EnableProEscalation: true,
}

// Google Driveフォルダ設定
var FolderIDs = map[string]string{
	"SOURCE":          "1T_XJURJbSsSiarr2Y-ofH0lCpSn9Dmak",
	"MONEY_TAX":       "1rUnmoPoJoD-UwLn0PQW7-FtBfg9FlUTi",
	"PROJECT_ASSET":   "1xBNSHmmnpuQpz0pvXxg_VlUAy0Zk4SOG",
	"LIFE_ADMIN":      "1keZdfSSrmpPqPWhC22Fg2A5GmaCfg3Xg",
	"CHILDREN_EDU":    "14TyZrKoXRSSP6kxpytxvap4poKmDn4qs",
	"PHOTO_OTHER":     "1euBhhNI0Ny13tXs1JVrcO0KLKHySFnEy",
	"LIBRARY":         "1MxppChMYZOJOyY2s-w6CsVam3P5_vccv",
	"NOTEBOOKLM_SYNC": "1AVRbK5Zy8IVC3XYtSQ7ZwNGMIB3ToaBu",
	"ARCHIVE":         "14iqjkHeBVMz47sNzPFkxrp5syr2tIOeO",
}

// カテゴリマッピング
var CategoryMap = map[string]string{
	"10_マネー・税務":    FolderIDs["MONEY_TAX"],
	"20_プロジェクト・資産": FolderIDs["PROJECT_ASSET"],
	"30_ライフ・行政":    FolderIDs["LIFE_ADMIN"],
	"40_子供・教育":     FolderIDs["CHILDREN_EDU"],
	"50_写真・その他":    FolderIDs["PHOTO_OTHER"],
	"90_ライブラリ":     FolderIDs["LIBRARY"],
	"99_転送済みアーカイブ": FolderIDs["ARCHIVE"],
}

// 子供の名寄せルール
var ChildAliases = map[string][]string{
	"明日香":  {"明日香", "あすか", "アスカ", "Asuka"},
	"遥香":   {"遥香", "はるか", "ハルカ", "Haruka"},
	"文香":   {"文香", "ふみか", "フミカ", "Fumika"},
	"ビクトル": {"ビクトル", "Victor", "Viktor"},
	"ミハイル": {"ミハイル", "Mikhail", "Mihail"},
	"アンナ":  {"アンナ", "Anna"},
}

// 大人の名寄せルール
var AdultAliases = map[string][]string{
	"千世己": {"千世己", "Chiseki", "ちせき", "チセキ"},
	"まどか": {"まどか", "Madoka", "マドカ"},
	"怜央奈": {"怜央奈", "Leo", "Reona", "れおな", "レオナ"},
	"今日子": {"今日子", "Kyoko", "きょうこ", "綿谷", "Wataya"},
	"えりか": {"えりか", "Erika", "エリカ", "Эрика"},
}

// 年度サブフォルダを作成するカテゴリ
var CategoriesWithYearSubfolder = []string{
	"10_マネー・税務",
	"30_ライフ・行政",
	"40_子供・教育",
}

// NotebookLM同期対象カテゴリ
var NotebookLMSyncCategories = []string{
	"10_マネー・税務",
	"20_プロジェクト・資産",
	"30_ライフ・行政",
	"40_子供・教育",
	"90_ライブラリ",
}

// NotebookLMドキュメントのオーナー
const NotebookLMOwnerEmail = "leo.courageous.lion@gmail.com"

// 子供の卒業設定
const ChildGraduationGrade = 12

// 大人用カテゴリ
var AdultCategories = []string{
	"10_マネー・税務",
	"30_ライフ・行政",
}

// 学年・クラス設定
type GradeConfig struct {
	BaseFiscalYear     int
	ChildrenBaseGrades map[string]int
	PreschoolClasses   map[int]PreschoolClass
	SharedGroups       map[string]SharedGroup
}

type PreschoolClass struct {
	Name  string
	Emoji string
}

type SharedGroup struct {
	Children   []string
	FolderName string
	Label      string
}

var GradeConfigSettings = GradeConfig{
	BaseFiscalYear: 2024,
	ChildrenBaseGrades: map[string]int{
		"ビクトル": 2,
		"明日香":  -1,
		"遥香":   -3,
		"アンナ":  -3,
		"ミハイル": -3,
		"文香":   -5,
	},
	PreschoolClasses: map[int]PreschoolClass{
		-1: {Name: "ぽぷら組", Emoji: "🌳"},
		-2: {Name: "いちょう組", Emoji: "🍂"},
		-3: {Name: "くるみ組", Emoji: "🐿️"},
		-4: {Name: "たんぽぽ組", Emoji: "🌼"},
		-5: {Name: "りんご組", Emoji: "🍎"},
		-6: {Name: "さくらんぼ組", Emoji: "🍒"},
	},
	SharedGroups: map[string]SharedGroup{
		"くるみ組": {
			Children:   []string{"遥香", "アンナ", "ミハイル"},
			FolderName: "Haruka-Anna-Mischa",
			Label:      "🐿️",
		},
		"いちょう組": {
			Children:   []string{"遥香", "アンナ", "ミハイル"},
			FolderName: "Haruka-Anna-Mischa",
			Label:      "🍂",
		},
		"ぽぷら組": {
			Children:   []string{"遥香", "アンナ", "ミハイル"},
			FolderName: "Haruka-Anna-Mischa",
			Label:      "🌳",
		},
	},
}

// サブカテゴリ
var SubCategories = []string{
	"01_お便り・スケジュール",
	"02_提出・手続き・重要",
	"03_記録・作品・成績",
}

// CalendarSync設定
var TargetSubfolderNames = []string{
	"01_お便り・スケジュール",
	"02_提出・手続き・重要",
}

var CalendarID = GetEnv("CALENDAR_ID", "639243bb722810f6fbe8f95b9dc57adf65677a53810d7fcdc76eef0fc4845792@group.calendar.google.com")

// API設定
type APIConfig struct {
	TimeoutMS    int
	MaxRetries   int
	RetryDelayMS int
}

var API = APIConfig{
	TimeoutMS:    30000,
	MaxRetries:   3,
	RetryDelayMS: 1000,
}

// 対応ファイル形式
var SupportedMimeTypes = []string{
	"application/pdf",
	"image/jpeg",
	"image/png",
	"image/gif",
	"image/bmp",
}

// 処理解像度（DPI）設定
var DPI = struct {
	Internal int // Gemini解析・OCR用
	Photos   int // Google Photosアップロード用
}{
	Internal: 200,
	Photos:   300,
}

// getEnv は環境変数を取得し、存在しない場合はデフォルト値を返す
func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
