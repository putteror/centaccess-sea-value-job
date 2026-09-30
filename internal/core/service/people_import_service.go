package service

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/putteror/sea-value/internal/core/port"
)

type PeopleImportService interface {
	ProcessPendingExcelFiles(ctx context.Context) error
}

type peopleImportServiceImpl struct {
	apiClient   port.CentAccessAPIPort
	excelReader port.ExcelReaderPort
}

func NewPeopleImportService(apiClient port.CentAccessAPIPort, excelReader port.ExcelReaderPort) PeopleImportService {
	return &peopleImportServiceImpl{
		apiClient:   apiClient,
		excelReader: excelReader,
	}
}

func (s *peopleImportServiceImpl) ProcessPendingExcelFiles(ctx context.Context) error {
	files, err := s.excelReader.ListPendingPeopleFiles()
	if err != nil {
		return fmt.Errorf("failed to scan people folder: %w", err)
	}

	if len(files) == 0 {
		log.Println("[TASK 3: PEOPLE IMPORT] No new excel files in people folder.")
		return nil
	}

	log.Printf("[TASK 3: PEOPLE IMPORT] Found %d file(s) to process.\n", len(files))

	for _, file := range files {
		log.Printf("[TASK 3] Processing File: %s...\n", file)

		lowerFile := strings.ToLower(file)
		// 1. If it's an Excel spreadsheet (.xlsx / .xls), upload directly to CentAccess People Import API
		if strings.HasSuffix(lowerFile, ".xlsx") || strings.HasSuffix(lowerFile, ".xls") {
			res, err := s.apiClient.UploadPeopleExcel(ctx, file, "EMPLOYEE")
			if err != nil {
				log.Printf("[TASK 3 ERROR] Failed to upload Excel file %s to CentAccess: %v\n", file, err)
				continue
			}

			log.Printf("[TASK 3 SUCCESS] File %s successfully imported: %s (Success: %d, Failed: %d)\n",
				file, res.Message, res.SuccessCount, res.FailedCount)
			_ = s.excelReader.MoveProcessedFile(file)
			continue
		}

		// 2. If it's CSV or other format, parse and use batch API
		peopleRows, err := s.excelReader.ReadPeopleExcel(file)
		if err != nil {
			log.Printf("[TASK 3 ERROR] Failed to parse file %s: %v\n", file, err)
			continue
		}

		if len(peopleRows) == 0 {
			log.Printf("[TASK 3 WARNING] File %s has no person rows.\n", file)
			_ = s.excelReader.MoveProcessedFile(file)
			continue
		}

		res, err := s.apiClient.ImportPeople(ctx, peopleRows)
		if err != nil {
			log.Printf("[TASK 3 ERROR] Failed to send import API for file %s: %v\n", file, err)
			continue
		}

		log.Printf("[TASK 3 SUCCESS] File %s processed: %d success, %d failed.\n", file, res.SuccessCount, res.FailedCount)
		_ = s.excelReader.MoveProcessedFile(file)
	}

	return nil
}
