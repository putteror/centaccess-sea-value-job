package model

// PeopleImportRow represents row data parsed from Excel file
type PeopleImportRow struct {
	FirstName    string   `json:"firstName"`
	LastName     string   `json:"lastName"`
	PersonCode   string   `json:"personCode"`
	CitizenID    string   `json:"citizenId,omitempty"`
	Department   string   `json:"department"`
	Position     string   `json:"position,omitempty"`
	Email        string   `json:"email,omitempty"`
	Phone        string   `json:"phone,omitempty"`
	CardNumber   string   `json:"cardNumber,omitempty"`
	FaceImageURL string   `json:"faceImageUrl,omitempty"`
}

// CreatePersonRequest represents single person creation payload matching POST /api/people
type CreatePersonRequest struct {
	FirstName           string   `json:"firstName"`
	LastName            string   `json:"lastName"`
	PersonType          string   `json:"personType"` // "EMPLOYEE", "VISITOR", "CONTRACTOR", "VIP"
	PersonCode          string   `json:"personCode,omitempty"`
	Department          string   `json:"department,omitempty"`
	JobPosition         string   `json:"jobPosition,omitempty"`
	MobileNumber        string   `json:"mobileNumber,omitempty"`
	Email               string   `json:"email,omitempty"`
	FaceImagePath       string   `json:"faceImagePath,omitempty"`
	AccessCardNumbers   []string `json:"accessCardNumbers,omitempty"`
	AccessControlRuleIDs []string `json:"accessControlRuleIds,omitempty"`
	LicensePlateTexts   []string `json:"licensePlateTexts,omitempty"`
}

// ImportResult represents response after pushing person or excel file to CentAccess
type ImportResult struct {
	TotalProcessed int      `json:"totalProcessed"`
	SuccessCount   int      `json:"successCount"`
	FailedCount    int      `json:"failedCount"`
	Message        string   `json:"message,omitempty"`
	Errors         []string `json:"errors,omitempty"`
}
