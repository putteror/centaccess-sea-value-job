package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

// WriteOvertimeRecords writes formatted text file (person_code scan_date scan_time device_name) in Bangkok timezone
func (s *FileStorageAdapter) WriteOvertimeRecords(ctx context.Context, records []model.AccessRecord, targetDate time.Time) (string, error) {
	folder := s.cfg.Jobs.Overtime.ExportFolder
	if folder == "" {
		folder = "overtime"
	}
	dirPath := filepath.Join(s.cfg.ExportBaseDir, folder)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create overtime folder: %w", err)
	}

	filename := model.GenerateTXTFilename("overtime", targetDate, targetDate)
	filePath := filepath.Join(dirPath, filename)

	content := model.FormatAccessRecordsToTXT(records)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return filePath, nil
}
