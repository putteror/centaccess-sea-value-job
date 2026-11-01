package service

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/putteror/sea-value/internal/core/port"
)

type PeopleImportService interface {
	ProcessPendingExcelFiles(ctx context.Context) error
}

type peopleImportServiceImpl struct {
	apiClient       port.CentAccessAPIPort
	excelReader     port.ExcelReaderPort
	deptRuleService port.DepartmentRuleService
}

func NewPeopleImportService(
	apiClient port.CentAccessAPIPort,
	excelReader port.ExcelReaderPort,
	deptRuleService port.DepartmentRuleService,
) PeopleImportService {
	return &peopleImportServiceImpl{
		apiClient:       apiClient,
		excelReader:     excelReader,
		deptRuleService: deptRuleService,
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
		fileName := filepath.Base(file)
		log.Printf("[TASK 3] Processing File: %s...\n", fileName)

		lowerFile := strings.ToLower(file)
		// 1. If it's an Excel spreadsheet (.xlsx / .xls), auto-enrich and upload directly to CentAccess
		if strings.HasSuffix(lowerFile, ".xlsx") || strings.HasSuffix(lowerFile, ".xls") {
			// Auto-enrich .xlsx with Rule based on Department and Category mapping before uploading
			if strings.HasSuffix(lowerFile, ".xlsx") && s.deptRuleService != nil {
				enriched, err := s.excelReader.EnrichExcelWithRules(file, s.deptRuleService.GetRuleCodeForDepartmentAndCategory)
				if err != nil {
					log.Printf("[TASK 3 WARNING] Failed to enrich %s with rules: %v (continuing upload)\n", fileName, err)
				} else if enriched > 0 {
					log.Printf("[TASK 3 ENRICH] Injected Rule into %d row(s) for %s based on Dept & Rules mapping.\n", enriched, fileName)
				}
			}

			res, err := s.apiClient.UploadPeopleExcel(ctx, file, "PERMANENT")
			if err != nil {
				log.Printf("[TASK 3 ERROR] Failed to upload Excel file %s to CentAccess: %v\n", fileName, err)
				continue
			}

			log.Printf("[TASK 3 SUCCESS] File %s successfully imported: %s (Success: %d, Failed: %d)\n",
				fileName, res.Message, res.SuccessCount, res.FailedCount)
			_ = s.excelReader.MoveProcessedFile(file)
			continue
		}

		// 2. If it's CSV or other format, parse and use batch API
		peopleRows, err := s.excelReader.ReadPeopleExcel(file)
		if err != nil {
			log.Printf("[TASK 3 ERROR] Failed to parse file %s: %v\n", fileName, err)
			continue
		}

		if len(peopleRows) == 0 {
			log.Printf("[TASK 3 WARNING] File %s has no person rows.\n", fileName)
			_ = s.excelReader.MoveProcessedFile(file)
			continue
		}

		// Enrich in-memory rows with rule if empty
		if s.deptRuleService != nil {
			enrichedCount := 0
			for i := range peopleRows {
				if peopleRows[i].RuleCode == "" && peopleRows[i].Department != "" {
					matchedRule := s.deptRuleService.GetRuleCodeForDepartmentAndCategory(peopleRows[i].Department, peopleRows[i].Category)
					if matchedRule != "" {
						peopleRows[i].RuleCode = matchedRule
						enrichedCount++
					}
				}
			}
			if enrichedCount > 0 {
				log.Printf("[TASK 3 ENRICH] Injected Rule into %d in-memory row(s) for %s\n", enrichedCount, fileName)
			}
		}

		res, err := s.apiClient.ImportPeople(ctx, peopleRows)
		if err != nil {
			log.Printf("[TASK 3 ERROR] Failed to send import API for file %s: %v\n", fileName, err)
			continue
		}

		log.Printf("[TASK 3 SUCCESS] File %s processed: %d success, %d failed.\n", fileName, res.SuccessCount, res.FailedCount)
		_ = s.excelReader.MoveProcessedFile(file)
	}

	return nil
}
