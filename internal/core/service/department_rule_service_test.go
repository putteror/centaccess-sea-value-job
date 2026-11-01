package service_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
	"github.com/putteror/sea-value/internal/core/service"
)

func TestDepartmentRuleService(t *testing.T) {
	tmpDir := t.TempDir()
	rulesFile := filepath.Join(tmpDir, "department_rules.json")

	cfg := &config.Config{
		DepartmentRulesFilePath: rulesFile,
	}

	deptService := service.NewDepartmentRuleService(cfg)
	ctx := context.Background()

	// 1. Initial should be empty
	rules, err := deptService.GetRules(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}

	// 2. Upsert Rule 1 (with Category MONTHLY)
	err = deptService.UpsertRule(ctx, model.DepartmentRule{
		DepartmentCode: "C-1111",
		Category:       "MONTHLY",
		RuleCode:       "RULE-01",
	})
	if err != nil {
		t.Fatalf("failed to upsert rule: %v", err)
	}

	// 3. Lookup Rule
	ruleCode := deptService.GetRuleCodeForDepartmentAndCategory("C-1111", "MONTHLY")
	if ruleCode != "RULE-01" {
		t.Errorf("expected RULE-01, got %s", ruleCode)
	}

	// 4. Update Rule 1
	err = deptService.UpsertRule(ctx, model.DepartmentRule{
		DepartmentCode: "C-1111",
		Category:       "MONTHLY",
		RuleCode:       "RULE-01-UPDATED",
	})
	if err != nil {
		t.Fatalf("failed to update rule: %v", err)
	}
	if code := deptService.GetRuleCodeForDepartmentAndCategory("C-1111", "MONTHLY"); code != "RULE-01-UPDATED" {
		t.Errorf("expected RULE-01-UPDATED, got %s", code)
	}

	// 5. Generate Excel Template
	tmplBytes, err := deptService.GenerateExcelTemplate()
	if err != nil {
		t.Fatalf("failed to generate template: %v", err)
	}
	if len(tmplBytes) == 0 {
		t.Errorf("expected template bytes")
	}

	// 6. Import from Template Excel
	inserted, updated, err := deptService.ImportFromExcel(ctx, bytes.NewReader(tmplBytes))
	if err != nil {
		t.Fatalf("failed to import excel template: %v", err)
	}
	// Template has C-1111/MONTHLY (which updates) + C-1111/DAILY (insert) + C-2222/default (insert)
	if inserted != 2 {
		t.Errorf("expected 2 inserted, got %d", inserted)
	}
	if updated != 1 {
		t.Errorf("expected 1 updated, got %d", updated)
	}

	// 7. Verify C-2222
	if code := deptService.GetRuleCodeForDepartment("C-2222"); code != "RULE-GENERAL" {
		t.Errorf("expected RULE-GENERAL for C-2222, got %s", code)
	}

	// 8. Export to Excel
	exportBytes, err := deptService.ExportToExcel(ctx)
	if err != nil {
		t.Fatalf("failed to export: %v", err)
	}
	if len(exportBytes) == 0 {
		t.Errorf("expected export bytes")
	}

	// 9. Delete C-1111 with all categories
	err = deptService.DeleteRule(ctx, "C-1111", "*")
	if err != nil {
		t.Fatalf("failed to delete rule: %v", err)
	}
	if code := deptService.GetRuleCodeForDepartment("C-1111"); code != "" {
		t.Errorf("expected empty rule for deleted dept, got %s", code)
	}

	// 10. Test Category-specific rules and fallback
	_ = deptService.UpsertRule(ctx, model.DepartmentRule{
		DepartmentCode: "DEPT-A",
		Category:       "MONTHLY",
		RuleCode:       "RULE-OFFICE",
	})
	_ = deptService.UpsertRule(ctx, model.DepartmentRule{
		DepartmentCode: "DEPT-A",
		Category:       "DAILY",
		RuleCode:       "RULE-PLANT",
	})
	_ = deptService.UpsertRule(ctx, model.DepartmentRule{
		DepartmentCode: "DEPT-A",
		Category:       "", // default
		RuleCode:       "RULE-DEFAULT",
	})

	// Exact match MONTHLY
	if code := deptService.GetRuleCodeForDepartmentAndCategory("DEPT-A", "MONTHLY"); code != "RULE-OFFICE" {
		t.Errorf("expected RULE-OFFICE, got %s", code)
	}
	// Exact match DAILY
	if code := deptService.GetRuleCodeForDepartmentAndCategory("DEPT-A", "DAILY"); code != "RULE-PLANT" {
		t.Errorf("expected RULE-PLANT, got %s", code)
	}
	// Fallback to default when category is unknown or empty
	if code := deptService.GetRuleCodeForDepartmentAndCategory("DEPT-A", "OTHER"); code != "RULE-DEFAULT" {
		t.Errorf("expected fallback RULE-DEFAULT, got %s", code)
	}
	if code := deptService.GetRuleCodeForDepartmentAndCategory("DEPT-A", ""); code != "RULE-DEFAULT" {
		t.Errorf("expected fallback RULE-DEFAULT for empty category, got %s", code)
	}

	// Delete only DAILY
	_ = deptService.DeleteRule(ctx, "DEPT-A", "DAILY")
	if code := deptService.GetRuleCodeForDepartmentAndCategory("DEPT-A", "DAILY"); code != "RULE-DEFAULT" {
		t.Errorf("expected fallback to default after DAILY deleted, got %s", code)
	}
	if code := deptService.GetRuleCodeForDepartmentAndCategory("DEPT-A", "MONTHLY"); code != "RULE-OFFICE" {
		t.Errorf("expected MONTHLY to remain, got %s", code)
	}

	// Verify persistence file exists
	if _, err := os.Stat(rulesFile); os.IsNotExist(err) {
		t.Errorf("department_rules.json was not created")
	}
}

func TestDepartmentRule_SpacesAndQuotesAndCaseInsensitive(t *testing.T) {
	tmpDir := t.TempDir()
	rulesFile := filepath.Join(tmpDir, "dept_rules_test_sanitize.json")

	cfg := &config.Config{
		DepartmentRulesFilePath: rulesFile,
	}
	deptService := service.NewDepartmentRuleService(cfg)
	ctx := context.Background()

	// Upsert with extra whitespace, surrounding quotes, mixed case
	err := deptService.UpsertRule(ctx, model.DepartmentRule{
		DepartmentCode: "  \"C-1111\"  ",
		Category:       "  \"monthly\"  ",
		RuleCode:       "  \"RULE-OFFICE\"  ",
	})
	if err != nil {
		t.Fatalf("failed to upsert sanitized rule: %v", err)
	}

	// 1. Lookup with uppercase, no quotes
	code1 := deptService.GetRuleCodeForDepartmentAndCategory("C-1111", "MONTHLY")
	if code1 != "RULE-OFFICE" {
		t.Errorf("expected RULE-OFFICE, got '%s'", code1)
	}

	// 2. Lookup with lowercase, spaces, and single quotes
	code2 := deptService.GetRuleCodeForDepartmentAndCategory("  'c-1111'  ", "  'Monthly'  ")
	if code2 != "RULE-OFFICE" {
		t.Errorf("expected RULE-OFFICE, got '%s'", code2)
	}

	// 3. Lookup with double quotes in department
	code3 := deptService.GetRuleCodeForDepartmentAndCategory("\"C-1111\"", "\"MONTHLY\"")
	if code3 != "RULE-OFFICE" {
		t.Errorf("expected RULE-OFFICE, got '%s'", code3)
	}
}

func TestDepartmentRule_EditCategoryUpdatesInsteadOfCreatingDuplicate(t *testing.T) {
	tmpDir := t.TempDir()
	rulesFile := filepath.Join(tmpDir, "dept_rules_test_edit.json")

	cfg := &config.Config{
		DepartmentRulesFilePath: rulesFile,
	}
	deptService := service.NewDepartmentRuleService(cfg)
	ctx := context.Background()

	// 1. Initial insert: C-1111 with DAILY
	err := deptService.UpsertRule(ctx, model.DepartmentRule{
		DepartmentCode: "C-1111",
		Category:       "DAILY",
		RuleCode:       "RULE-01",
	})
	if err != nil {
		t.Fatalf("failed to insert initial rule: %v", err)
	}

	rules, _ := deptService.GetRules(ctx)
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}

	// 2. Edit rule: change Category from DAILY to MONTHLY, Rule to RULE-UPDATED
	err = deptService.UpsertRule(ctx, model.DepartmentRule{
		OrigDepartmentCode: "C-1111",
		OrigCategory:       "DAILY",
		DepartmentCode:     "C-1111",
		Category:           "MONTHLY",
		RuleCode:           "RULE-UPDATED",
	})
	if err != nil {
		t.Fatalf("failed to edit rule: %v", err)
	}

	rules, _ = deptService.GetRules(ctx)
	if len(rules) != 1 {
		t.Errorf("expected 1 rule (updated in-place), but got %d rules (duplicate created!)", len(rules))
	}

	// 3. DAILY should not exist anymore
	dailyCode := deptService.GetRuleCodeForDepartmentAndCategory("C-1111", "DAILY")
	if dailyCode != "" {
		t.Errorf("expected old DAILY rule to be gone, got '%s'", dailyCode)
	}

	// 4. MONTHLY should exist with new code
	monthlyCode := deptService.GetRuleCodeForDepartmentAndCategory("C-1111", "MONTHLY")
	if monthlyCode != "RULE-UPDATED" {
		t.Errorf("expected MONTHLY rule to be 'RULE-UPDATED', got '%s'", monthlyCode)
	}
}

