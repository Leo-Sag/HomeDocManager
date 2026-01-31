package service

import (
	"strconv"
	"strings"
	"time"

	"github.com/leo-sagawa/homedocmanager/internal/config"
)

// GradeManager は学年管理サービス
type GradeManager struct{}

// NewGradeManager は新しいGradeManagerを作成
func NewGradeManager() *GradeManager {
	return &GradeManager{}
}

// CalculateFiscalYear は日付文字列から年度を計算
func (gm *GradeManager) CalculateFiscalYear(dateStr string) int {
	if dateStr == "" {
		// 現在の日付から計算
		now := time.Now()
		year := now.Year()
		if now.Month() >= 4 {
			return year
		}
		return year - 1
	}

	// YYYYMMDD形式をパース
	if len(dateStr) < 8 {
		return time.Now().Year()
	}

	year, err := strconv.Atoi(dateStr[:4])
	if err != nil {
		return time.Now().Year()
	}

	month, err := strconv.Atoi(dateStr[4:6])
	if err != nil {
		return year
	}

	// 4月以降は当該年度、3月以前は前年度
	if month >= 4 {
		return year
	}
	return year - 1
}

// IdentifyChildren は学年・クラス情報から子供を特定
func (gm *GradeManager) IdentifyChildren(gradeClassText string, fiscalYear int) []string {
	var children []string

	// 1. まず個別の子供の学年から判定（より具体的な指定を優先）
	for childName, baseGrade := range config.GradeConfigSettings.ChildrenBaseGrades {
		yearDiff := fiscalYear - config.GradeConfigSettings.BaseFiscalYear
		currentGrade := baseGrade + yearDiff

		// 学年文字列から判定（例：「小2」「年長」）
		if gm.matchesGrade(gradeClassText, currentGrade) {
			children = append(children, childName)
		}
	}

	// 個別の一致があった場合はそれを返す
	if len(children) > 0 {
		return children
	}

	// 2. 個別の一致がない場合のみ、共有グループのクラス名から判定
	for className, group := range config.GradeConfigSettings.SharedGroups {
		if strings.Contains(gradeClassText, className) {
			return group.Children
		}
	}

	return children
}

// matchesGrade は学年文字列が一致するかチェック
func (gm *GradeManager) matchesGrade(text string, grade int) bool {
	// 小学校の場合
	if grade >= 1 && grade <= 6 {
		gradeStr := strconv.Itoa(grade)
		if strings.Contains(text, "小"+gradeStr) || strings.Contains(text, gradeStr+"年生") {
			return true
		}
	}

	// 保育園の場合
	if classInfo, exists := config.GradeConfigSettings.PreschoolClasses[grade]; exists {
		if strings.Contains(text, classInfo.Name) {
			return true
		}
	}

	return false
}

// IsGraduated は高校卒業しているかチェック
func (gm *GradeManager) IsGraduated(childName string, fiscalYear int) bool {
	baseGrade, exists := config.GradeConfigSettings.ChildrenBaseGrades[childName]
	if !exists {
		return false
	}

	yearDiff := fiscalYear - config.GradeConfigSettings.BaseFiscalYear
	currentGrade := baseGrade + yearDiff

	return currentGrade > config.ChildGraduationGrade
}

// GetChildGrade は子供の現在の学年を取得
func (gm *GradeManager) GetChildGrade(childName string, fiscalYear int) int {
	baseGrade, exists := config.GradeConfigSettings.ChildrenBaseGrades[childName]
	if !exists {
		return 0
	}

	yearDiff := fiscalYear - config.GradeConfigSettings.BaseFiscalYear
	return baseGrade + yearDiff
}

// GetGradeInfo は学年情報（ラベルと絵文字）を取得
func (gm *GradeManager) GetGradeInfo(grade int) (string, string) {
	// 保育園の場合
	if classInfo, exists := config.GradeConfigSettings.PreschoolClasses[grade]; exists {
		return classInfo.Name, classInfo.Emoji
	}

	// 小学校の場合
	if grade >= 1 && grade <= 6 {
		return "小" + strconv.Itoa(grade), ""
	}

	// 中学校の場合
	if grade >= 7 && grade <= 9 {
		return "中" + strconv.Itoa(grade-6), ""
	}

	// 高校の場合
	if grade >= 10 && grade <= 12 {
		return "高" + strconv.Itoa(grade-9), ""
	}

	return "", ""
}

// ResolveFolderName は複数の子供から共有フォルダ名を解決
func (gm *GradeManager) ResolveFolderName(children []string) (string, string, string) {
	if len(children) == 0 {
		return "", "", ""
	}

	// 共有グループに該当するかチェック
	for _, group := range config.GradeConfigSettings.SharedGroups {
		if gm.matchesGroup(children, group.Children) {
			return group.FolderName, group.Label, group.Label
		}
	}

	// 単一の子供の場合
	if len(children) == 1 {
		return children[0], children[0], ""
	}

	// 複数の子供だが共有グループではない場合
	return "", "", ""
}

// matchesGroup は子供リストがグループに一致するかチェック
func (gm *GradeManager) matchesGroup(children, groupChildren []string) bool {
	if len(children) != len(groupChildren) {
		return false
	}

	childMap := make(map[string]bool)
	for _, child := range children {
		childMap[child] = true
	}

	for _, groupChild := range groupChildren {
		if !childMap[groupChild] {
			return false
		}
	}

	return true
}
