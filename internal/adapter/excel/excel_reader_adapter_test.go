package excel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/putteror/sea-value/internal/config"
	"github.com/xuri/excelize/v2"
)

func TestEnrichExcelWithRules_AddNewColumn(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "employees.xlsx")

	f := excelize.NewFile()
	sheet := "Sheet1"
	_ = f.SetCellValue(sheet, "A1", "First Name")
	_ = f.SetCellValue(sheet, "B1", "Last Name")
	_ = f.SetCellValue(sheet, "C1", "Department")

	// Row 2
	_ = f.SetCellValue(sheet, "A2", "John")
	_ = f.SetCellValue(sheet, "B2", "Doe")
	_ = f.SetCellValue(sheet, "C2", "C-1111")

	// Row 3
	_ = f.SetCellValue(sheet, "A3", "Jane")
	_ = f.SetCellValue(sheet, "B3", "Smith")
	_ = f.SetCellValue(sheet, "C3", "D-2222")

	// Row 4
	_ = f.SetCellValue(sheet, "A4", "Bob")
	_ = f.SetCellValue(sheet, "B4", "Taylor")
	_ = f.SetCellValue(sheet, "C4", "UNKNOWN")

	if err := f.SaveAs(filePath); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	_ = f.Close()

	ruleLookup := func(deptCode, category string) string {
		switch deptCode {
		case "C-1111":
			return "RULE-A"
		case "D-2222":
			return "RULE-B"
		default:
			return ""
		}
	}

	adapter := NewExcelReaderAdapter(&config.Config{})
	enriched, err := adapter.EnrichExcelWithRules(filePath, ruleLookup)
	if err != nil {
		t.Fatalf("EnrichExcelWithRules failed: %v", err)
	}

	if enriched != 2 {
		t.Errorf("expected 2 enriched rows, got %d", enriched)
	}

	// Verify enriched file contents
	fCheck, err := excelize.OpenFile(filePath)
	if err != nil {
		t.Fatalf("failed to reopen file: %v", err)
	}
	defer fCheck.Close()

	headerRule, _ := fCheck.GetCellValue(sheet, "D1")
	if headerRule != "Rule" {
		t.Errorf("expected header D1 to be 'Rule', got '%s'", headerRule)
	}

	ruleRow2, _ := fCheck.GetCellValue(sheet, "D2")
	if ruleRow2 != "RULE-A" {
		t.Errorf("expected D2 to be 'RULE-A', got '%s'", ruleRow2)
	}

	ruleRow3, _ := fCheck.GetCellValue(sheet, "D3")
	if ruleRow3 != "RULE-B" {
		t.Errorf("expected D3 to be 'RULE-B', got '%s'", ruleRow3)
	}

	ruleRow4, _ := fCheck.GetCellValue(sheet, "D4")
	if ruleRow4 != "" {
		t.Errorf("expected D4 to be empty, got '%s'", ruleRow4)
	}
}

func TestEnrichExcelWithRules_ExistingColumn(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "employees_with_rule.xlsx")

	f := excelize.NewFile()
	sheet := "Sheet1"
	_ = f.SetCellValue(sheet, "A1", "ชื่อ")
	_ = f.SetCellValue(sheet, "B1", "แผนก")
	_ = f.SetCellValue(sheet, "C1", "สิทธิ์การเข้าออก")

	// Row 2: Already has rule, do not overwrite
	_ = f.SetCellValue(sheet, "A2", "Somchai")
	_ = f.SetCellValue(sheet, "B2", "C-1111")
	_ = f.SetCellValue(sheet, "C2", "CUSTOM-RULE")

	// Row 3: Empty rule, should be enriched
	_ = f.SetCellValue(sheet, "A3", "Somsri")
	_ = f.SetCellValue(sheet, "B3", "C-1111")
	_ = f.SetCellValue(sheet, "C3", "")

	if err := f.SaveAs(filePath); err != nil {
		t.Fatalf("failed to save test excel: %v", err)
	}
	_ = f.Close()

	ruleLookup := func(deptCode, category string) string {
		if deptCode == "C-1111" {
			return "RULE-AUTO"
		}
		return ""
	}

	adapter := NewExcelReaderAdapter(&config.Config{})
	enriched, err := adapter.EnrichExcelWithRules(filePath, ruleLookup)
	if err != nil {
		t.Fatalf("failed to enrich: %v", err)
	}

	if enriched != 1 {
		t.Errorf("expected 1 enriched row, got %d", enriched)
	}

	fCheck, _ := excelize.OpenFile(filePath)
	defer fCheck.Close()

	r2, _ := fCheck.GetCellValue(sheet, "C2")
	if r2 != "CUSTOM-RULE" {
		t.Errorf("expected C2 to retain 'CUSTOM-RULE', got '%s'", r2)
	}

	r3, _ := fCheck.GetCellValue(sheet, "C3")
	if r3 != "RULE-AUTO" {
		t.Errorf("expected C3 to be populated with 'RULE-AUTO', got '%s'", r3)
	}
}

func TestEnrichExcelWithRules_DepartmentAndCategory(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "employees_with_category.xlsx")

	f := excelize.NewFile()
	sheet := "Sheet1"
	_ = f.SetCellValue(sheet, "A1", "ชื่อ")
	_ = f.SetCellValue(sheet, "B1", "แผนก")
	_ = f.SetCellValue(sheet, "C1", "ประเภท")

	// Row 2: C-1111 + MONTHLY
	_ = f.SetCellValue(sheet, "A2", "User 1")
	_ = f.SetCellValue(sheet, "B2", "C-1111")
	_ = f.SetCellValue(sheet, "C2", "MONTHLY")

	// Row 3: C-1111 + DAILY
	_ = f.SetCellValue(sheet, "A3", "User 2")
	_ = f.SetCellValue(sheet, "B3", "C-1111")
	_ = f.SetCellValue(sheet, "C3", "DAILY")

	// Row 4: C-1111 + OTHER (should get fallback)
	_ = f.SetCellValue(sheet, "A4", "User 3")
	_ = f.SetCellValue(sheet, "B4", "C-1111")
	_ = f.SetCellValue(sheet, "C4", "TEMP")

	_ = f.SaveAs(filePath)
	_ = f.Close()

	ruleLookup := func(deptCode, category string) string {
		if deptCode == "C-1111" {
			if category == "MONTHLY" {
				return "RULE-OFFICE"
			}
			if category == "DAILY" {
				return "RULE-PLANT"
			}
			return "RULE-DEFAULT"
		}
		return ""
	}

	adapter := NewExcelReaderAdapter(&config.Config{})
	enriched, err := adapter.EnrichExcelWithRules(filePath, ruleLookup)
	if err != nil {
		t.Fatalf("failed to enrich: %v", err)
	}
	if enriched != 3 {
		t.Errorf("expected 3 enriched rows, got %d", enriched)
	}

	fCheck, _ := excelize.OpenFile(filePath)
	defer fCheck.Close()

	r1, _ := fCheck.GetCellValue(sheet, "D1")
	if r1 != "Rule" {
		t.Errorf("expected header D1 to be 'Rule', got '%s'", r1)
	}

	r2, _ := fCheck.GetCellValue(sheet, "D2")
	if r2 != "RULE-OFFICE" {
		t.Errorf("expected D2 to be 'RULE-OFFICE', got '%s'", r2)
	}

	r3, _ := fCheck.GetCellValue(sheet, "D3")
	if r3 != "RULE-PLANT" {
		t.Errorf("expected D3 to be 'RULE-PLANT', got '%s'", r3)
	}

	r4, _ := fCheck.GetCellValue(sheet, "D4")
	if r4 != "RULE-DEFAULT" {
		t.Errorf("expected D4 to be 'RULE-DEFAULT', got '%s'", r4)
	}
}

func TestReadPeopleExcel_Parsing(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "people.csv")

	csvContent := "First Name,Last Name,Employee ID,Department,Rule,Card Number\nJohn,Doe,EMP001,IT,RULE-01,123456\n"
	if err := os.WriteFile(filePath, []byte(csvContent), 0644); err != nil {
		t.Fatalf("failed to write csv: %v", err)
	}

	adapter := NewExcelReaderAdapter(&config.Config{})
	rows, err := adapter.ReadPeopleExcel(filePath)
	if err != nil {
		t.Fatalf("ReadPeopleExcel failed: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].FirstName != "John" || rows[0].PersonCode != "EMP001" || rows[0].RuleCode != "RULE-01" {
		t.Errorf("unexpected parsed row: %+v", rows[0])
	}
}

