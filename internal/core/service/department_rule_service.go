package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
	"github.com/putteror/sea-value/internal/core/port"
	"github.com/xuri/excelize/v2"
)

type departmentRuleServiceImpl struct {
	cfg *config.Config
}

// NewDepartmentRuleService creates a new instance of DepartmentRuleService
func NewDepartmentRuleService(cfg *config.Config) port.DepartmentRuleService {
	return &departmentRuleServiceImpl{
		cfg: cfg,
	}
}

func (s *departmentRuleServiceImpl) GetRules(ctx context.Context) ([]model.DepartmentRule, error) {
	return s.cfg.GetDepartmentRules()
}

func cleanRuleVal(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "\"'\uFEFF\u200B")
	return strings.TrimSpace(s)
}

func (s *departmentRuleServiceImpl) UpsertRule(ctx context.Context, rule model.DepartmentRule) error {
	deptCode := cleanRuleVal(rule.DepartmentCode)
	category := cleanRuleVal(rule.Category)
	ruleCode := cleanRuleVal(rule.RuleCode)
	origDept := cleanRuleVal(rule.OrigDepartmentCode)
	origCat := cleanRuleVal(rule.OrigCategory)

	if deptCode == "" {
		return fmt.Errorf("department code is required")
	}
	if ruleCode == "" {
		return fmt.Errorf("rule code is required")
	}

	rules, err := s.cfg.GetDepartmentRules()
	if err != nil {
		rules = []model.DepartmentRule{}
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")

	// If editing an existing record (origDept is specified)
	if origDept != "" {
		var newRules []model.DepartmentRule
		for _, r := range rules {
			// Skip the original record being edited
			if strings.EqualFold(cleanRuleVal(r.DepartmentCode), origDept) && strings.EqualFold(cleanRuleVal(r.Category), origCat) {
				continue
			}
			// Skip any existing duplicate with the new key so we don't end up with duplicate (dept, cat)
			if strings.EqualFold(cleanRuleVal(r.DepartmentCode), deptCode) && strings.EqualFold(cleanRuleVal(r.Category), category) {
				continue
			}
			newRules = append(newRules, r)
		}
		newRules = append(newRules, model.DepartmentRule{
			DepartmentCode: deptCode,
			Category:       category,
			RuleCode:       ruleCode,
			UpdatedAt:      nowStr,
		})
		rules = newRules
	} else {
		// New entry or upsert by target key
		found := false
		for i := range rules {
			deptMatch := strings.EqualFold(cleanRuleVal(rules[i].DepartmentCode), deptCode)
			catMatch := strings.EqualFold(cleanRuleVal(rules[i].Category), category)
			if deptMatch && catMatch {
				rules[i].DepartmentCode = deptCode
				rules[i].Category = category
				rules[i].RuleCode = ruleCode
				rules[i].UpdatedAt = nowStr
				found = true
				break
			}
		}

		if !found {
			rules = append(rules, model.DepartmentRule{
				DepartmentCode: deptCode,
				Category:       category,
				RuleCode:       ruleCode,
				UpdatedAt:      nowStr,
			})
		}
	}

	// Ensure Orig fields are stripped
	for i := range rules {
		rules[i].OrigDepartmentCode = ""
		rules[i].OrigCategory = ""
	}

	return s.cfg.SaveDepartmentRules(rules)
}

func (s *departmentRuleServiceImpl) DeleteRule(ctx context.Context, deptCode, category string) error {
	targetDept := cleanRuleVal(deptCode)
	if targetDept == "" {
		return fmt.Errorf("department code is required")
	}
	targetCat := cleanRuleVal(category)

	rules, err := s.cfg.GetDepartmentRules()
	if err != nil {
		return err
	}

	var filtered []model.DepartmentRule
	for _, r := range rules {
		deptMatch := strings.EqualFold(cleanRuleVal(r.DepartmentCode), targetDept)
		if !deptMatch {
			filtered = append(filtered, r)
			continue
		}

		// If targetCat is "*", delete all rules under this department
		if targetCat == "*" {
			continue
		}

		// Otherwise, delete exact matching category
		catMatch := strings.EqualFold(cleanRuleVal(r.Category), targetCat)
		if !catMatch {
			filtered = append(filtered, r)
		}
	}

	return s.cfg.SaveDepartmentRules(filtered)
}

func (s *departmentRuleServiceImpl) ImportFromExcel(ctx context.Context, r io.Reader) (inserted, updated int, err error) {
	f, err := excelize.OpenReader(r)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to open excel file: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return 0, 0, fmt.Errorf("no sheets found in excel file")
	}

	sheetName := sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read sheet %s: %w", sheetName, err)
	}

	if len(rows) == 0 {
		return 0, 0, fmt.Errorf("excel sheet is empty")
	}

	deptColIdx := 0
	catColIdx := -1
	ruleColIdx := 1
	startRowIdx := 0

	// Inspect header row if present
	firstRow := rows[0]
	hasHeader := false
	for idx, col := range firstRow {
		val := strings.ToLower(strings.TrimSpace(col))
		valClean := strings.ReplaceAll(strings.ReplaceAll(val, " ", ""), "_", "")
		if strings.Contains(valClean, "dept") || strings.Contains(valClean, "แผนก") || strings.Contains(valClean, "department") {
			deptColIdx = idx
			hasHeader = true
		} else if strings.Contains(valClean, "cat") || strings.Contains(valClean, "หมวด") || strings.Contains(valClean, "ประเภท") || strings.Contains(valClean, "กลุ่ม") {
			catColIdx = idx
			hasHeader = true
		} else if strings.Contains(valClean, "rule") || strings.Contains(valClean, "กฎ") || strings.Contains(valClean, "สิทธิ์") || strings.Contains(valClean, "กะ") {
			ruleColIdx = idx
			hasHeader = true
		}
	}

	if hasHeader {
		startRowIdx = 1
	}

	currentRules, err := s.cfg.GetDepartmentRules()
	if err != nil {
		currentRules = []model.DepartmentRule{}
	}

	// composite key -> index
	makeKey := func(dept, cat string) string {
		return fmt.Sprintf("%s|%s", strings.ToLower(cleanRuleVal(dept)), strings.ToLower(cleanRuleVal(cat)))
	}

	ruleMap := make(map[string]int)
	for i, r := range currentRules {
		ruleMap[makeKey(r.DepartmentCode, r.Category)] = i
	}

	nowStr := time.Now().Format("2006-01-02 15:04:05")

	for i := startRowIdx; i < len(rows); i++ {
		row := rows[i]
		if len(row) <= deptColIdx {
			continue
		}

		deptCode := cleanRuleVal(row[deptColIdx])
		if deptCode == "" {
			continue
		}

		var category string
		if catColIdx >= 0 && len(row) > catColIdx {
			category = cleanRuleVal(row[catColIdx])
		}

		var ruleCode string
		if len(row) > ruleColIdx {
			ruleCode = cleanRuleVal(row[ruleColIdx])
		}

		if ruleCode == "" {
			continue
		}

		key := makeKey(deptCode, category)
		if existingIdx, exists := ruleMap[key]; exists {
			currentRules[existingIdx].DepartmentCode = deptCode
			currentRules[existingIdx].Category = category
			currentRules[existingIdx].RuleCode = ruleCode
			currentRules[existingIdx].UpdatedAt = nowStr
			updated++
		} else {
			currentRules = append(currentRules, model.DepartmentRule{
				DepartmentCode: deptCode,
				Category:       category,
				RuleCode:       ruleCode,
				UpdatedAt:      nowStr,
			})
			ruleMap[key] = len(currentRules) - 1
			inserted++
		}
	}

	if err := s.cfg.SaveDepartmentRules(currentRules); err != nil {
		return inserted, updated, fmt.Errorf("failed to save imported rules: %w", err)
	}

	return inserted, updated, nil
}

func (s *departmentRuleServiceImpl) GenerateExcelTemplate() ([]byte, error) {
	f := excelize.NewFile()
	sheetName := "DepartmentRules"
	f.SetSheetName("Sheet1", sheetName)

	// Set Headers
	f.SetCellValue(sheetName, "A1", "Department Code")
	f.SetCellValue(sheetName, "B1", "Category")
	f.SetCellValue(sheetName, "C1", "Rule Code")

	// Set Sample Rows
	f.SetCellValue(sheetName, "A2", "C-1111")
	f.SetCellValue(sheetName, "B2", "MONTHLY")
	f.SetCellValue(sheetName, "C2", "RULE-OFFICE-01")

	f.SetCellValue(sheetName, "A3", "C-1111")
	f.SetCellValue(sheetName, "B3", "DAILY")
	f.SetCellValue(sheetName, "C3", "RULE-PLANT-02")

	f.SetCellValue(sheetName, "A4", "C-2222")
	f.SetCellValue(sheetName, "B4", "") // default
	f.SetCellValue(sheetName, "C4", "RULE-GENERAL")

	// Set Column widths
	_ = f.SetColWidth(sheetName, "A", "A", 22)
	_ = f.SetColWidth(sheetName, "B", "B", 20)
	_ = f.SetColWidth(sheetName, "C", "C", 22)

	// Header style
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{
			Bold:  true,
			Color: "FFFFFF",
		},
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"2563EB"}, // Blue-600
			Pattern: 1,
		},
		Alignment: &excelize.Alignment{
			Horizontal: "center",
			Vertical:   "center",
		},
	})
	_ = f.SetCellStyle(sheetName, "A1", "C1", headerStyle)

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *departmentRuleServiceImpl) ExportToExcel(ctx context.Context) ([]byte, error) {
	rules, err := s.cfg.GetDepartmentRules()
	if err != nil {
		return nil, err
	}

	f := excelize.NewFile()
	sheetName := "DepartmentRules"
	f.SetSheetName("Sheet1", sheetName)

	f.SetCellValue(sheetName, "A1", "Department Code")
	f.SetCellValue(sheetName, "B1", "Category")
	f.SetCellValue(sheetName, "C1", "Rule Code")
	f.SetCellValue(sheetName, "D1", "Last Updated")

	_ = f.SetColWidth(sheetName, "A", "A", 22)
	_ = f.SetColWidth(sheetName, "B", "B", 20)
	_ = f.SetColWidth(sheetName, "C", "C", 22)
	_ = f.SetColWidth(sheetName, "D", "D", 22)

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"1E293B"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	_ = f.SetCellStyle(sheetName, "A1", "D1", headerStyle)

	for i, r := range rules {
		row := i + 2
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", row), r.DepartmentCode)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", row), r.Category)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", row), r.RuleCode)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", row), r.UpdatedAt)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (s *departmentRuleServiceImpl) GetRuleCodeForDepartment(deptCode string) string {
	return s.cfg.GetRuleCodeForDepartment(deptCode)
}

func (s *departmentRuleServiceImpl) GetRuleCodeForDepartmentAndCategory(deptCode, category string) string {
	return s.cfg.GetRuleCodeForDepartmentAndCategory(deptCode, category)
}
