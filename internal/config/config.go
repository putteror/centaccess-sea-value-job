package config

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

type Config struct {
	mu sync.RWMutex

	// From Environment Variables (.env)
	AppAPIURL      string `json:"appApiUrl"`
	AppOvertimeURL string `json:"appOvertimeUrl"`
	AppAPIKey      string `json:"appApiKey"`
	AppUsername    string `json:"appUsername"`
	AppPassword    string `json:"appPassword"`
	AppTenantID    string `json:"appTenantId"`
	AppSecretKey   string `json:"appSecretKey"`
	ConfigFilePath string `json:"-"`
	DevicesFilePath string `json:"-"`

	// From config.json (persisted)
	HTTPPort      string             `json:"httpPort"`
	ExportBaseDir string             `json:"exportBaseDir"`
	LogLevel      string             `json:"logLevel"`
	AppToken      string             `json:"appToken,omitempty"`
	Schedules     []ScheduleConfig   `json:"schedules"`
	Jobs          JobsConfig         `json:"jobs"`
	Devices       []model.DeviceItem `json:"devices,omitempty"`
}

// loadDotEnv parses .env files and sets environment variables if not already set
func loadDotEnv(filepaths ...string) {
	for _, fp := range filepaths {
		f, err := os.Open(fp)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, `"'`)
				if _, exists := os.LookupEnv(key); !exists || os.Getenv(key) == "" {
					_ = os.Setenv(key, val)
				}
			}
		}
		_ = f.Close()
	}
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return fallback
}

func LoadConfig() (*Config, error) {
	loadDotEnv(".env", ".env.local", ".env.example")

	apiURL := getEnv("APP_API_URL", "http://localhost:8080")
	overtimeURL := getEnv("APP_OVERTIME_URL", apiURL)
	apiKey := getEnv("APP_API_KEY", "")
	username := getEnv("APP_USERNAME", "")
	password := getEnv("APP_PASSWORD", "")
	tenantID := getEnv("APP_TENANT_ID", "")
	secretKey := getEnv("APP_SECRET_KEY", "")
	configFilePath := getEnv("CONFIG_FILE_PATH", "./config.json")

	if absPath, err := filepath.Abs(configFilePath); err == nil {
		configFilePath = absPath
	}

	devicesFilePath := getEnv("DEVICES_FILE_PATH", filepath.Join(filepath.Dir(configFilePath), "devices.json"))
	if absPath, err := filepath.Abs(devicesFilePath); err == nil {
		devicesFilePath = absPath
	}

	if apiURL == "" {
		return nil, fmt.Errorf("APP_API_URL is required in environment")
	}

	fileBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", configFilePath, err)
	}

	var jsonCfg JSONConfigFile
	if err := json.Unmarshal(fileBytes, &jsonCfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", configFilePath, err)
	}

	if jsonCfg.HTTPPort == "" {
		jsonCfg.HTTPPort = getEnv("HTTP_PORT", "9090")
	}
	if jsonCfg.ExportBaseDir == "" {
		jsonCfg.ExportBaseDir = "./data"
	}

	// Load devices from devices.json
	var loadedDevices []model.DeviceItem
	if devBytes, err := os.ReadFile(devicesFilePath); err == nil {
		_ = json.Unmarshal(devBytes, &loadedDevices)
	} else {
		// Initialize empty devices.json if missing
		emptyBytes, _ := json.MarshalIndent([]model.DeviceItem{}, "", "  ")
		_ = os.WriteFile(devicesFilePath, emptyBytes, 0644)
	}

	// Migrate or set default schedules if empty
	if len(jsonCfg.Schedules) == 0 {
		jsonCfg.Schedules = []ScheduleConfig{
			{
				ID:           "sch_att_1",
				Name:         "Attendance & OT Gate - Every 2 Hours",
				Type:         "attendance",
				CronSchedule: "0 */2 * * *",
				Enabled:      true,
			},
			{
				ID:           "sch_att_2",
				Name:         "Attendance & OT Gate - Daily Morning (05:30)",
				Type:         "attendance",
				CronSchedule: "30 5 * * *",
				Enabled:      true,
			},
			{
				ID:           "sch_help_1",
				Name:         "Help Records - Daily Sync (23:00)",
				Type:         "help",
				CronSchedule: "0 23 * * *",
				Enabled:      true,
			},
			{
				ID:           "sch_people_1",
				Name:         "People Excel - Auto Scan (15m)",
				Type:         "people",
				CronSchedule: "*/15 * * * *",
				Enabled:      true,
			},
		}
	}

	return &Config{
		AppAPIURL:       apiURL,
		AppOvertimeURL:  overtimeURL,
		AppAPIKey:       apiKey,
		AppUsername:     username,
		AppPassword:     password,
		AppTenantID:     tenantID,
		AppSecretKey:    secretKey,
		ConfigFilePath:  configFilePath,
		DevicesFilePath: devicesFilePath,
		HTTPPort:        jsonCfg.HTTPPort,
		ExportBaseDir:   jsonCfg.ExportBaseDir,
		LogLevel:        jsonCfg.LogLevel,
		AppToken:        jsonCfg.AppToken,
		Schedules:       jsonCfg.Schedules,
		Jobs:            jsonCfg.Jobs,
		Devices:         loadedDevices,
	}, nil
}

// SetAppToken saves the active JWT token into memory and config.json
func (c *Config) SetAppToken(token string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.AppToken = token

	fileBytes, err := os.ReadFile(c.ConfigFilePath)
	if err != nil {
		return err
	}

	var jsonCfg JSONConfigFile
	if err := json.Unmarshal(fileBytes, &jsonCfg); err != nil {
		return err
	}

	jsonCfg.AppToken = token
	data, err := json.MarshalIndent(jsonCfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(c.ConfigFilePath, data, 0644)
}

// GetDevices returns thread-safe copy of persisted devices
func (c *Config) GetDevices() []model.DeviceItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	res := make([]model.DeviceItem, len(c.Devices))
	copy(res, c.Devices)
	return res
}

// SaveDevices updates the device registry and persists to devices.json and config.json
func (c *Config) SaveDevices(devices []model.DeviceItem) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.Devices = devices

	// 1. Persist directly to devices.json
	devData, err := json.MarshalIndent(devices, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize devices: %w", err)
	}
	if err := os.WriteFile(c.DevicesFilePath, devData, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", c.DevicesFilePath, err)
	}

	// 2. Update targetDeviceIds, targetDeviceCodes and targetDeviceNames in Jobs config as well
	var otIds []string
	var otCodes []string
	for _, d := range devices {
		if d.IsOT {
			if d.ID != "" {
				otIds = append(otIds, d.ID)
			}
			code := d.GetCode()
			if code != "" {
				otCodes = append(otCodes, code)
			}
		}
	}

	c.Jobs.Overtime.TargetDeviceIDs = otIds
	c.Jobs.Overtime.TargetDeviceCodes = otCodes
	c.Jobs.Attendance.ExcludeDeviceIDs = otIds
	c.Jobs.Attendance.ExcludeDeviceCodes = otCodes

	fileBytes, err := os.ReadFile(c.ConfigFilePath)
	if err == nil {
		var jsonCfg JSONConfigFile
		if err := json.Unmarshal(fileBytes, &jsonCfg); err == nil {
			jsonCfg.Jobs.Overtime.TargetDeviceIDs = otIds
			jsonCfg.Jobs.Overtime.TargetDeviceCodes = otCodes
			jsonCfg.Jobs.Attendance.ExcludeDeviceIDs = otIds
			jsonCfg.Jobs.Attendance.ExcludeDeviceCodes = otCodes
			if cfgData, err := json.MarshalIndent(jsonCfg, "", "  "); err == nil {
				_ = os.WriteFile(c.ConfigFilePath, cfgData, 0644)
			}
		}
	}

	return nil
}

// GetInitialSyncTime returns the starting time when a device/service has never synced or is reset.
// If InitialLookbackHours > 0, returns now - initialLookbackHours.
// If InitialLookbackHours <= 0, returns the start of the current day (00:00:00).
func (c *Config) GetInitialSyncTime(serviceType string) time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	lookbackHours := 0

	switch strings.ToLower(serviceType) {
	case "help":
		lookbackHours = c.Jobs.Help.InitialLookbackHours
	case "overtime":
		lookbackHours = c.Jobs.Overtime.InitialLookbackHours
		if lookbackHours == 0 {
			lookbackHours = c.Jobs.Attendance.InitialLookbackHours
		}
	default: // attendance / access_records
		lookbackHours = c.Jobs.Attendance.InitialLookbackHours
	}

	if lookbackHours > 0 {
		return now.Add(-time.Duration(lookbackHours) * time.Hour)
	}

	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// GetMaxLookbackDays returns the maximum safety lookback window in days (defaults to 7 if unconfigured or <= 0).
func (c *Config) GetMaxLookbackDays(serviceType string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	days := 0
	switch strings.ToLower(serviceType) {
	case "help":
		days = c.Jobs.Help.MaxLookbackDays
	case "overtime":
		days = c.Jobs.Overtime.MaxLookbackDays
		if days == 0 {
			days = c.Jobs.Attendance.MaxLookbackDays
		}
	default:
		days = c.Jobs.Attendance.MaxLookbackDays
	}

	if days <= 0 {
		return 7
	}
	return days
}

// GetEarliestDeviceSyncTime calculates the earliest lastSyncTime across enabled devices for a given service type ("attendance" or "help")
func (c *Config) GetEarliestDeviceSyncTime(serviceType string) (time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.Devices) == 0 {
		return time.Time{}, false
	}

	var earliest time.Time
	hasAnySync := false
	now := time.Now()
	maxDays := 7
	switch strings.ToLower(serviceType) {
	case "help":
		if c.Jobs.Help.MaxLookbackDays > 0 {
			maxDays = c.Jobs.Help.MaxLookbackDays
		}
	case "overtime":
		if c.Jobs.Overtime.MaxLookbackDays > 0 {
			maxDays = c.Jobs.Overtime.MaxLookbackDays
		} else if c.Jobs.Attendance.MaxLookbackDays > 0 {
			maxDays = c.Jobs.Attendance.MaxLookbackDays
		}
	default:
		if c.Jobs.Attendance.MaxLookbackDays > 0 {
			maxDays = c.Jobs.Attendance.MaxLookbackDays
		}
	}
	maxLookback := now.AddDate(0, 0, -maxDays)

	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	isHelp := strings.EqualFold(serviceType, "help")

	for _, d := range c.Devices {
		// Only consider enabled devices
		if !d.Enabled {
			continue
		}

		timeStr := d.AttendanceLastSyncTime
		if isHelp {
			timeStr = d.HelpLastSyncTime
		} else if strings.TrimSpace(timeStr) == "" {
			timeStr = d.LastSyncTime // fallback to legacy field
		}

		if strings.TrimSpace(timeStr) == "" {
			// If an active device has never synced for this service, return false so caller can fallback to GetInitialSyncTime
			return time.Time{}, false
		}

		for _, fmtStr := range formats {
			if t, err := time.Parse(fmtStr, strings.TrimSpace(timeStr)); err == nil {
				if !hasAnySync || t.Before(earliest) {
					earliest = t
					hasAnySync = true
				}
				break
			}
		}
	}

	if !hasAnySync {
		return time.Time{}, false
	}

	// Safety cap: don't look back further than maxLookback
	if earliest.Before(maxLookback) {
		earliest = maxLookback
	}

	return earliest, true
}

// UpdateDevicesAttendanceSyncTime updates attendanceLastSyncTime for each device to the completed sync window end
func (c *Config) UpdateDevicesAttendanceSyncTime(deviceMaxTimes map[string]time.Time, fallbackTime time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.Devices) == 0 {
		return nil
	}

	for i := range c.Devices {
		if !c.Devices[i].Enabled {
			continue
		}

		devID := c.Devices[i].ID
		devCode := c.Devices[i].GetCode()
		devName := c.Devices[i].Name

		targetTime := fallbackTime
		if maxT, exists := deviceMaxTimes[devID]; exists && maxT.After(targetTime) {
			targetTime = maxT
		} else if maxT, exists := deviceMaxTimes[devCode]; devCode != "" && exists && maxT.After(targetTime) {
			targetTime = maxT
		} else if maxT, exists := deviceMaxTimes[devName]; exists && maxT.After(targetTime) {
			targetTime = maxT
		}

		c.Devices[i].AttendanceLastSyncTime = targetTime.Format(time.RFC3339)
		c.Devices[i].LastSyncTime = targetTime.Format(time.RFC3339)
	}

	// Persist to devices.json
	devData, err := json.MarshalIndent(c.Devices, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize devices: %w", err)
	}
	return os.WriteFile(c.DevicesFilePath, devData, 0644)
}

// UpdateDevicesHelpSyncTime updates helpLastSyncTime for each device to the completed sync window end
func (c *Config) UpdateDevicesHelpSyncTime(deviceMaxTimes map[string]time.Time, fallbackTime time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.Devices) == 0 {
		return nil
	}

	for i := range c.Devices {
		if !c.Devices[i].Enabled {
			continue
		}

		devID := c.Devices[i].ID
		devCode := c.Devices[i].GetCode()
		devName := c.Devices[i].Name

		targetTime := fallbackTime
		if maxT, exists := deviceMaxTimes[devID]; exists && maxT.After(targetTime) {
			targetTime = maxT
		} else if maxT, exists := deviceMaxTimes[devCode]; devCode != "" && exists && maxT.After(targetTime) {
			targetTime = maxT
		} else if maxT, exists := deviceMaxTimes[devName]; exists && maxT.After(targetTime) {
			targetTime = maxT
		}

		c.Devices[i].HelpLastSyncTime = targetTime.Format(time.RFC3339)
	}

	// Persist to devices.json
	devData, err := json.MarshalIndent(c.Devices, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize devices: %w", err)
	}
	return os.WriteFile(c.DevicesFilePath, devData, 0644)
}

// SetDeviceSyncTime updates or resets specific watermark timestamps for a device or all devices.
func (c *Config) SetDeviceSyncTime(deviceID string, attendanceTimeStr *string, helpTimeStr *string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.Devices {
		if deviceID == "" || c.Devices[i].ID == deviceID || (c.Devices[i].GetCode() != "" && c.Devices[i].GetCode() == deviceID) {
			if attendanceTimeStr != nil {
				c.Devices[i].AttendanceLastSyncTime = *attendanceTimeStr
				c.Devices[i].LastSyncTime = *attendanceTimeStr
			}
			if helpTimeStr != nil {
				c.Devices[i].HelpLastSyncTime = *helpTimeStr
			}
		}
	}

	devData, err := json.MarshalIndent(c.Devices, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize devices: %w", err)
	}
	return os.WriteFile(c.DevicesFilePath, devData, 0644)
}

// ResetDeviceSyncTime resets device watermarks ("attendance", "help", or "all")
func (c *Config) ResetDeviceSyncTime(deviceID string, syncType string, resetTime *time.Time) error {
	timeStr := ""
	if resetTime != nil {
		timeStr = resetTime.Format(time.RFC3339)
	}

	targetType := strings.ToLower(syncType)
	switch targetType {
	case "attendance":
		return c.SetDeviceSyncTime(deviceID, &timeStr, nil)
	case "help":
		return c.SetDeviceSyncTime(deviceID, nil, &timeStr)
	default:
		return c.SetDeviceSyncTime(deviceID, &timeStr, &timeStr)
	}
}

// RecordScheduleLastRun records the execution timestamp of a cron schedule in memory
func (c *Config) RecordScheduleLastRun(scheduleID string, runTime time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	timeStr := runTime.Format(time.RFC3339)
	for i := range c.Schedules {
		if c.Schedules[i].ID == scheduleID {
			c.Schedules[i].LastRunAt = timeStr
		}
	}
}

// SaveConfig updates config.json file on disk safely
func (c *Config) SaveConfig(updatedJSON JSONConfigFile) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if updatedJSON.AppToken == "" && c.AppToken != "" {
		updatedJSON.AppToken = c.AppToken
	}

	data, err := json.MarshalIndent(updatedJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.WriteFile(c.ConfigFilePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config to file %s: %w", c.ConfigFilePath, err)
	}

	c.HTTPPort = updatedJSON.HTTPPort
	c.ExportBaseDir = updatedJSON.ExportBaseDir
	c.LogLevel = updatedJSON.LogLevel
	c.AppToken = updatedJSON.AppToken
	c.Schedules = updatedJSON.Schedules
	c.Jobs = updatedJSON.Jobs

	return nil
}

