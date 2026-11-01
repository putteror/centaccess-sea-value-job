package service

import (
	"context"
	"io"
	"testing"

	"github.com/putteror/sea-value/internal/core/model"
)

type mockExcelReader struct {
	files            []string
	movedFiles       []string
	enrichCalledWith string
	enrichedReturn   int
}

func (m *mockExcelReader) ListPendingPeopleFiles() ([]string, error) {
	return m.files, nil
}

func (m *mockExcelReader) ReadPeopleExcel(filePath string) ([]model.PeopleImportRow, error) {
	return []model.PeopleImportRow{}, nil
}

func (m *mockExcelReader) MoveProcessedFile(filePath string) error {
	m.movedFiles = append(m.movedFiles, filePath)
	return nil
}

func (m *mockExcelReader) EnrichExcelWithRules(filePath string, ruleLookup func(deptCode, category string) string) (int, error) {
	m.enrichCalledWith = filePath
	if ruleLookup != nil {
		_ = ruleLookup("C-1111", "")
	}
	return m.enrichedReturn, nil
}

type mockDeptRuleService struct {
	rules map[string]string
}

func (m *mockDeptRuleService) GetRules(ctx context.Context) ([]model.DepartmentRule, error) {
	var list []model.DepartmentRule
	for k, v := range m.rules {
		list = append(list, model.DepartmentRule{DepartmentCode: k, RuleCode: v})
	}
	return list, nil
}

func (m *mockDeptRuleService) UpsertRule(ctx context.Context, rule model.DepartmentRule) error {
	m.rules[rule.DepartmentCode] = rule.RuleCode
	return nil
}

func (m *mockDeptRuleService) DeleteRule(ctx context.Context, deptCode, category string) error {
	delete(m.rules, deptCode)
	return nil
}

func (m *mockDeptRuleService) ImportFromExcel(ctx context.Context, file io.Reader) (int, int, error) {
	return 0, 0, nil
}

func (m *mockDeptRuleService) GenerateExcelTemplate() ([]byte, error) {
	return nil, nil
}

func (m *mockDeptRuleService) ExportToExcel(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (m *mockDeptRuleService) GetRuleCodeForDepartment(deptCode string) string {
	return m.rules[deptCode]
}

func (m *mockDeptRuleService) GetRuleCodeForDepartmentAndCategory(deptCode, category string) string {
	return m.rules[deptCode]
}

func TestPeopleImportService_AutoEnrichAndUpload(t *testing.T) {
	mockClient := &mockCentAccessClient{}
	mockReader := &mockExcelReader{
		files:          []string{"/tmp/people/test_employee.xlsx"},
		enrichedReturn: 5,
	}
	mockDeptService := &mockDeptRuleService{
		rules: map[string]string{
			"C-1111": "RULE-TEST",
		},
	}

	service := NewPeopleImportService(mockClient, mockReader, mockDeptService)
	err := service.ProcessPendingExcelFiles(context.Background())
	if err != nil {
		t.Fatalf("ProcessPendingExcelFiles failed: %v", err)
	}

	if mockReader.enrichCalledWith != "/tmp/people/test_employee.xlsx" {
		t.Errorf("expected EnrichExcelWithRules to be called for file, got '%s'", mockReader.enrichCalledWith)
	}

	if len(mockReader.movedFiles) != 1 || mockReader.movedFiles[0] != "/tmp/people/test_employee.xlsx" {
		t.Errorf("expected file to be moved after processing, got: %v", mockReader.movedFiles)
	}
}
