package model

// DepartmentRule represents the mapping between a CentAccess Department Code, optional Category, and a Sea Value Rule Code
type DepartmentRule struct {
	OrigDepartmentCode string `json:"origDepartmentCode,omitempty"`
	OrigCategory       string `json:"origCategory,omitempty"`
	DepartmentCode     string `json:"departmentCode"`
	Category           string `json:"category,omitempty"`
	RuleCode           string `json:"ruleCode"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
}

