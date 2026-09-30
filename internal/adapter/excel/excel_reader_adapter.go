package excel

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
	"github.com/putteror/sea-value/internal/core/port"
)

type ExcelReaderAdapter struct {
	cfg *config.Config
}

func NewExcelReaderAdapter(cfg *config.Config) port.ExcelReaderPort {
	return &ExcelReaderAdapter{
		cfg: cfg,
	}
}

// ListPendingPeopleFiles scans the configured people directory for unprocessed .xlsx/.xls/.csv files
func (r *ExcelReaderAdapter) ListPendingPeopleFiles() ([]string, error) {
	inputFolder := r.cfg.Jobs.PeopleImport.InputFolder
	if inputFolder == "" {
		inputFolder = "people"
	}

	peopleDir := filepath.Join(r.cfg.ExportBaseDir, inputFolder)
	if err := os.MkdirAll(peopleDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create people folder: %w", err)
	}

	entries, err := os.ReadDir(peopleDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		nameLower := strings.ToLower(entry.Name())
		if strings.HasSuffix(nameLower, ".xlsx") || strings.HasSuffix(nameLower, ".xls") || strings.HasSuffix(nameLower, ".csv") {
			files = append(files, filepath.Join(peopleDir, entry.Name()))
		}
	}

	return files, nil
}

// ReadPeopleExcel provides row data if local parsing is needed
func (r *ExcelReaderAdapter) ReadPeopleExcel(filePath string) ([]model.PeopleImportRow, error) {
	// Ready for parsing with excelize (github.com/xuri/excelize/v2) or CSV fallback
	return []model.PeopleImportRow{}, nil
}

// MoveProcessedFile moves finished file to a 'processed' archive subfolder
func (r *ExcelReaderAdapter) MoveProcessedFile(filePath string) error {
	archiveFolder := r.cfg.Jobs.PeopleImport.ArchiveFolder
	if archiveFolder == "" {
		archiveFolder = filepath.Join("people", "processed")
	}

	processedDir := filepath.Join(r.cfg.ExportBaseDir, archiveFolder)
	if err := os.MkdirAll(processedDir, 0755); err != nil {
		return err
	}

	fileName := filepath.Base(filePath)
	timestamp := time.Now().Format("20060102_150405")
	destPath := filepath.Join(processedDir, fmt.Sprintf("%s_%s", timestamp, fileName))

	return os.Rename(filePath, destPath)
}
