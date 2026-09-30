package config

// AttendanceJobConfig defines settings specific to attendance sync
type AttendanceJobConfig struct {
	Enabled              bool     `json:"enabled,omitempty"`
	CronSchedule         string   `json:"cronSchedule,omitempty"`
	ExportFolder         string   `json:"exportFolder"`
	FetchLimit           int      `json:"fetchLimit"`
	Format               string   `json:"format"`
	InitialLookbackHours int      `json:"initialLookbackHours,omitempty"` // Lookback hours when watermark is empty/never (e.g. 24, or 0 = start of day 00:00)
	MaxLookbackDays      int      `json:"maxLookbackDays,omitempty"`      // Max safety cap days to look back (default 7 days)
	ExcludeDeviceNames   []string `json:"excludeDeviceNames,omitempty"`
	ExcludeDeviceIDs     []string `json:"excludeDeviceIds,omitempty"`
}

// OvertimeJobConfig defines settings specific to overtime (access records filtered by OT devices)
type OvertimeJobConfig struct {
	Enabled              bool     `json:"enabled,omitempty"`
	CronSchedule         string   `json:"cronSchedule,omitempty"`
	ExportFolder         string   `json:"exportFolder"`
	Format               string   `json:"format"`
	InitialLookbackHours int      `json:"initialLookbackHours,omitempty"`
	MaxLookbackDays      int      `json:"maxLookbackDays,omitempty"`
	TargetDeviceNames    []string `json:"targetDeviceNames,omitempty"`
	TargetDeviceIDs      []string `json:"targetDeviceIds,omitempty"`
}

// HelpJobConfig defines settings specific to CentAccess Help / Overtime records API
type HelpJobConfig struct {
	Enabled              bool   `json:"enabled,omitempty"`
	CronSchedule         string `json:"cronSchedule,omitempty"`
	ExportFolder         string `json:"exportFolder"`
	Format               string `json:"format"`
	InitialLookbackHours int    `json:"initialLookbackHours,omitempty"`
	MaxLookbackDays      int    `json:"maxLookbackDays,omitempty"`
}

// PeopleImportJobConfig defines settings specific to excel people import
type PeopleImportJobConfig struct {
	Enabled       bool   `json:"enabled,omitempty"`
	CronSchedule  string `json:"cronSchedule,omitempty"`
	InputFolder   string `json:"inputFolder"`
	ArchiveFolder string `json:"archiveFolder"`
	BatchSize     int    `json:"batchSize"`
}

// JobsConfig groups all job-specific configurations
type JobsConfig struct {
	Attendance   AttendanceJobConfig   `json:"attendance"`
	Overtime     OvertimeJobConfig     `json:"overtime"`
	Help         HelpJobConfig         `json:"help"`
	PeopleImport PeopleImportJobConfig `json:"peopleImport"`
}
