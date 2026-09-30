package config

// JSONConfigFile represents the persistent config.json file schema on disk
type JSONConfigFile struct {
	HTTPPort      string           `json:"httpPort"`
	ExportBaseDir string           `json:"exportBaseDir"`
	LogLevel      string           `json:"logLevel"`
	AppToken      string           `json:"appToken,omitempty"`
	Schedules     []ScheduleConfig `json:"schedules"`
	Jobs          JobsConfig       `json:"jobs"`
}

