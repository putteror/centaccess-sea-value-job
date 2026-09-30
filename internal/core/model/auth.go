package model

// LoginRequest represents payload sent to /api/login
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Permission represents user permission details
type Permission struct {
	Resource string   `json:"resource"`
	Actions  []string `json:"actions"`
}

// LoginResponseData represents data payload from /api/login
type LoginResponseData struct {
	Token            string       `json:"token"`
	Username         string       `json:"username"`
	Role             string       `json:"role"`
	DefaultDashboard string       `json:"defaultDashboard"`
	TenantID         string       `json:"tenantId"`
	TenantName       string       `json:"tenantName"`
	TenantLevel      string       `json:"tenantLevel"`
	Features         []string     `json:"features"`
	Permissions      []Permission `json:"permissions"`
}
