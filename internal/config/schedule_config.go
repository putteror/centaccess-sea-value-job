package config

// ScheduleConfig represents an individual dynamic cron job schedule
type ScheduleConfig struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Type         string `json:"type"` // "attendance" | "overtime" | "help" | "people"
	CronSchedule string `json:"cronSchedule"`
	Enabled      bool   `json:"enabled"`
	LastRunAt    string `json:"lastRunAt,omitempty"` // Audit timestamp of last execution
}

// GetSchedules returns a copy of active schedules in a thread-safe manner
func (c *Config) GetSchedules() []ScheduleConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()

	res := make([]ScheduleConfig, len(c.Schedules))
	copy(res, c.Schedules)
	return res
}
