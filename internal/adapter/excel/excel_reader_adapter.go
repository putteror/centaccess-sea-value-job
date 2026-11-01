package excel

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
	"github.com/putteror/sea-value/internal/core/port"
	"github.com/xuri/excelize/v2"
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

// EnrichExcelWithRules inspects an Excel file (.xlsx), locates the department and category columns,
// and auto-populates the rule column based on mappings using the provided ruleLookup function.
// If the Rule column is missing, it dynamically adds the "Rule" header.
func (r *ExcelReaderAdapter) EnrichExcelWithRules(filePath string, ruleLookup func(deptCode, category string) string) (int, error) {
	if ruleLookup == nil {
		return 0, nil
	}

	lower := strings.ToLower(filePath)
	if !strings.HasSuffix(lower, ".xlsx") {
		// Only .xlsx is supported for direct in-place enrichment via excelize
		return 0, nil
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to open excel file %s: %w", filePath, err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return 0, nil
	}
	sheetName := sheets[0]

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, fmt.Errorf("failed to get rows from sheet %s: %w", sheetName, err)
	}
	if len(rows) <= 1 {
		// Empty or header only
		return 0, nil
	}

	cleanCell := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.Trim(s, "\"'\uFEFF\u200B")
		return strings.TrimSpace(s)
	}

	cleanHeader := func(s string) string {
		s = strings.ToLower(cleanCell(s))
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "_", "")
		s = strings.ReplaceAll(s, "-", "")
		return s
	}

	isDeptHeader := func(h string) bool {
		ch := cleanHeader(h)
		return ch == "department" || ch == "dept" || ch == "departmentcode" || ch == "deptcode" ||
			ch == "แผนก" || ch == "รหัสแผนก" || ch == "ฝ่าย" || ch == "รหัสฝ่าย" || ch == "หน่วยงาน" ||
			ch == "persondepartment" || ch == "persondepartmentcode"
	}

	isCategoryHeader := func(h string) bool {
		ch := cleanHeader(h)
		return ch == "category" || ch == "cat" || ch == "categorycode" || ch == "employeecategory" || ch == "personcategory" ||
			ch == "หมวดหมู่" || ch == "หมวด" || ch == "ประเภท" || ch == "กลุ่ม" || ch == "ประเภทพนักงาน" || ch == "กลุ่มพนักงาน" || ch == "สายงาน"
	}

	isRuleHeader := func(h string) bool {
		ch := cleanHeader(h)
		return ch == "rule" || ch == "rulecode" || ch == "rules" || ch == "accessrule" ||
			ch == "accesscontrolrule" || ch == "accesscontrolruleid" || ch == "accesscontrolrules" ||
			ch == "กฎ" || ch == "รหัสกฎ" || ch == "สิทธิ์" || ch == "รหัสสิทธิ์" || ch == "สิทธิ์การเข้าออก"
	}

	headerRow := rows[0]
	deptColIdx := -1
	catColIdx := -1
	ruleColIdx := -1

	for i, col := range headerRow {
		if isDeptHeader(col) && deptColIdx == -1 {
			deptColIdx = i
		}
		if isCategoryHeader(col) && catColIdx == -1 {
			catColIdx = i
		}
		if isRuleHeader(col) && ruleColIdx == -1 {
			ruleColIdx = i
		}
	}

	if deptColIdx == -1 {
		log.Printf("[PEOPLE EXCEL] No department column found in %s, skipping auto-enrichment\n", filepath.Base(filePath))
		return 0, nil
	}

	addedRuleColumn := false
	if ruleColIdx == -1 {
		ruleColIdx = len(headerRow)
		colName, err := excelize.ColumnNumberToName(ruleColIdx + 1)
		if err != nil {
			return 0, fmt.Errorf("failed to calculate column name for rule header: %w", err)
		}
		headerCell := fmt.Sprintf("%s1", colName)
		_ = f.SetCellValue(sheetName, headerCell, "Rule")
		addedRuleColumn = true
	}

	enrichedCount := 0
	for rIdx := 1; rIdx < len(rows); rIdx++ {
		rowNum := rIdx + 1
		deptVal := ""
		if deptColIdx < len(rows[rIdx]) {
			deptVal = cleanCell(rows[rIdx][deptColIdx])
		}

		catVal := ""
		if catColIdx >= 0 && catColIdx < len(rows[rIdx]) {
			catVal = cleanCell(rows[rIdx][catColIdx])
		}

		existingRule := ""
		if ruleColIdx < len(rows[rIdx]) {
			existingRule = cleanCell(rows[rIdx][ruleColIdx])
		}

		if existingRule == "" && deptVal != "" {
			matchedRule := ruleLookup(deptVal, catVal)
			if matchedRule != "" {
				cellRef, err := excelize.CoordinatesToCellName(ruleColIdx+1, rowNum)
				if err == nil {
					_ = f.SetCellValue(sheetName, cellRef, matchedRule)
					enrichedCount++
				}
			}
		}
	}

	if enrichedCount > 0 || addedRuleColumn {
		if err := f.Save(); err != nil {
			return enrichedCount, fmt.Errorf("failed to save enriched excel file %s: %w", filePath, err)
		}
	}

	return enrichedCount, nil
}

// ReadPeopleExcel provides row data if local parsing is needed (supports .xlsx and .csv)
func (r *ExcelReaderAdapter) ReadPeopleExcel(filePath string) ([]model.PeopleImportRow, error) {
	lower := strings.ToLower(filePath)
	if strings.HasSuffix(lower, ".xlsx") {
		f, err := excelize.OpenFile(filePath)
		if err != nil {
			return nil, err
		}
		defer f.Close()

		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return []model.PeopleImportRow{}, nil
		}
		rows, err := f.GetRows(sheets[0])
		if err != nil {
			return nil, err
		}
		return parseRowsToPeople(rows), nil
	}

	if strings.HasSuffix(lower, ".csv") {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		reader := csv.NewReader(file)
		rows, err := reader.ReadAll()
		if err != nil {
			return nil, err
		}
		return parseRowsToPeople(rows), nil
	}

	return []model.PeopleImportRow{}, nil
}

func parseRowsToPeople(rows [][]string) []model.PeopleImportRow {
	if len(rows) <= 1 {
		return []model.PeopleImportRow{}
	}

	clean := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		s = strings.ReplaceAll(s, " ", "")
		s = strings.ReplaceAll(s, "_", "")
		s = strings.ReplaceAll(s, "-", "")
		return s
	}

	header := rows[0]
	firstIdx := -1
	lastIdx := -1
	typeIdx := -1
	codeIdx := -1
	deptIdx := -1
	catIdx := -1
	ruleIdx := -1
	posIdx := -1
	emailIdx := -1
	phoneIdx := -1
	cardIdx := -1

	for i, h := range header {
		ch := clean(h)
		switch {
		case ch == "firstname" || ch == "first_name" || ch == "ชื่อ":
			firstIdx = i
		case ch == "lastname" || ch == "last_name" || ch == "นามสกุล":
			lastIdx = i
		case ch == "persontype" || ch == "person_type" || ch == "ประเภทบุคคล":
			typeIdx = i
		case ch == "personcode" || ch == "employeeid" || ch == "code" || ch == "รหัสพนักงาน" || ch == "รหัส":
			codeIdx = i
		case ch == "department" || ch == "dept" || ch == "แผนก" || ch == "ฝ่าย" || ch == "หน่วยงาน":
			deptIdx = i
		case ch == "category" || ch == "cat" || ch == "หมวดหมู่" || ch == "หมวด" || ch == "ประเภท" || ch == "กลุ่ม" || ch == "ประเภทพนักงาน":
			catIdx = i
		case ch == "rule" || ch == "rulecode" || ch == "accessrule" || ch == "กฎ" || ch == "สิทธิ์":
			ruleIdx = i
		case ch == "position" || ch == "jobposition" || ch == "ตำแหน่ง":
			posIdx = i
		case ch == "email" || ch == "อีเมล":
			emailIdx = i
		case ch == "phone" || ch == "tel" || ch == "mobilenumber" || ch == "เบอร์โทร":
			phoneIdx = i
		case ch == "cardnumber" || ch == "card" || ch == "บัตร" || ch == "หมายเลขบัตร":
			cardIdx = i
		}
	}

	var results []model.PeopleImportRow
	cleanVal := func(s string) string {
		s = strings.TrimSpace(s)
		s = strings.Trim(s, "\"'\uFEFF\u200B")
		return strings.TrimSpace(s)
	}

	for rIdx := 1; rIdx < len(rows); rIdx++ {
		row := rows[rIdx]
		getVal := func(idx int) string {
			if idx >= 0 && idx < len(row) {
				return cleanVal(row[idx])
			}
			return ""
		}

		firstName := getVal(firstIdx)
		lastName := getVal(lastIdx)
		personCode := getVal(codeIdx)

		// Basic check for non-empty row
		if firstName == "" && lastName == "" && personCode == "" {
			continue
		}

		results = append(results, model.PeopleImportRow{
			FirstName:  firstName,
			LastName:   lastName,
			PersonType: getVal(typeIdx),
			PersonCode: personCode,
			Department: getVal(deptIdx),
			Category:   getVal(catIdx),
			RuleCode:   getVal(ruleIdx),
			Position:   getVal(posIdx),
			Email:      getVal(emailIdx),
			Phone:      getVal(phoneIdx),
			CardNumber: getVal(cardIdx),
		})
	}

	return results
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
