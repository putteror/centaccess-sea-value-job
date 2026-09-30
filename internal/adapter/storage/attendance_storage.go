package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

// WriteAttendanceRecords writes formatted text file (person_code scan_date scan_time device_name) in Bangkok timezone
func (s *FileStorageAdapter) WriteAttendanceRecords(ctx context.Context, records []model.AccessRecord, targetDate time.Time) (string, error) {
	folder := s.cfg.Jobs.Attendance.ExportFolder
	if folder == "" {
		folder = "attendance"
	}
	dirPath := filepath.Join(s.cfg.ExportBaseDir, folder)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create attendance folder: %w", err)
	}

	filename := model.GenerateTXTFilename("attendance", targetDate, targetDate)
	filePath := filepath.Join(dirPath, filename)

	content := model.FormatAccessRecordsToTXT(records)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return filePath, nil
}
