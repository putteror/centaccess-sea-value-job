package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

// WriteHelpRecords writes formatted text file (person_code check_in_date check_in_time check_out_date check_out_time device_name help_department) in Bangkok timezone
func (s *FileStorageAdapter) WriteHelpRecords(ctx context.Context, records []model.HelpRecord, targetDate time.Time) (string, error) {
	folder := s.cfg.Jobs.Help.ExportFolder
	if folder == "" {
		folder = "help"
	}
	dirPath := filepath.Join(s.cfg.ExportBaseDir, folder)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create help folder: %w", err)
	}

	filename := model.GenerateTXTFilename("help", targetDate, targetDate)
	filePath := filepath.Join(dirPath, filename)

	content := model.FormatHelpRecordsToTXT(records, targetDate)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return filePath, nil
}
