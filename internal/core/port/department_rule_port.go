package port

import (
	"context"
	"io"

	"github.com/putteror/sea-value/internal/core/model"
)

// DepartmentRuleService defines business operations for managing Department Code and Category to Rule Code mappings
type DepartmentRuleService interface {
	GetRules(ctx context.Context) ([]model.DepartmentRule, error)
	UpsertRule(ctx context.Context, rule model.DepartmentRule) error
	DeleteRule(ctx context.Context, deptCode, category string) error
	ImportFromExcel(ctx context.Context, r io.Reader) (inserted, updated int, err error)
	GenerateExcelTemplate() ([]byte, error)
	ExportToExcel(ctx context.Context) ([]byte, error)
	GetRuleCodeForDepartment(deptCode string) string
	GetRuleCodeForDepartmentAndCategory(deptCode, category string) string
}

