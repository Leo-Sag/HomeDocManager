package service

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

// PDFProcessor はPDF処理を行うサービス
type PDFProcessor struct{}

// NewPDFProcessor は新しいPDFProcessorを作成
func NewPDFProcessor() *PDFProcessor {
	return &PDFProcessor{}
}

// IsPDF はMIMEタイプがPDFかどうかを判定
func (p *PDFProcessor) IsPDF(mimeType string) bool {
	return mimeType == "application/pdf"
}

// ConvertPDFToImages はPDFを画像（JPEG）に変換
func (p *PDFProcessor) ConvertPDFToImages(pdfBytes []byte, dpi int) ([][]byte, error) {
	// 一時ディレクトリ作成
	tmpDir, err := os.MkdirTemp("", "pdfconv-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// PDFを一時ファイルに保存
	pdfPath := filepath.Join(tmpDir, "input.pdf")
	if err := os.WriteFile(pdfPath, pdfBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to write temp pdf: %w", err)
	}

	// pdftoppm を実行 (JPEG形式、指定のdpi)
	// pdftoppm -jpeg -r [dpi] input.pdf output
	outputPrefix := filepath.Join(tmpDir, "page")
	cmd := exec.Command("pdftoppm", "-jpeg", "-r", fmt.Sprintf("%d", dpi), pdfPath, outputPrefix)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdftoppm failed: %v, stderr: %s", err, stderr.String())
	}

	// 生成された画像ファイルを読み込み
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read temp dir: %w", err)
	}

	var images [][]byte
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".jpg" {
			imgPath := filepath.Join(tmpDir, f.Name())
			imgData, err := os.ReadFile(imgPath)
			if err != nil {
				log.Printf("Warning: failed to read image file %s: %v", imgPath, err)
				continue
			}
			images = append(images, imgData)
		}
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("no images generated from PDF")
	}

	log.Printf("PDFを%d枚の画像に変換しました", len(images))
	return images, nil
}
