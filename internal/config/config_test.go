package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

func TestDevicesJsonSeparation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "config_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfgPath := filepath.Join(tmpDir, "config.json")
	devPath := filepath.Join(tmpDir, "devices.json")

	initialCfg := JSONConfigFile{
		HTTPPort:      "9090",
		ExportBaseDir: "./data",
		LogLevel:      "info",
		Jobs: JobsConfig{
			Attendance: AttendanceJobConfig{MaxLookbackDays: 60},
			Help:       HelpJobConfig{MaxLookbackDays: 60},
		},
		Schedules: []ScheduleConfig{
			{ID: "sch_1", Name: "Schedule 1", Type: "attendance", CronSchedule: "0 */2 * * *", Enabled: true},
		},
	}
	cfgBytes, _ := json.MarshalIndent(initialCfg, "", "  ")
	_ = os.WriteFile(cfgPath, cfgBytes, 0644)

	_ = os.Setenv("APP_API_URL", "http://localhost:8080")
	_ = os.Setenv("CONFIG_FILE_PATH", cfgPath)
	_ = os.Setenv("DEVICES_FILE_PATH", devPath)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig error: %v", err)
	}

	if len(cfg.GetDevices()) != 0 {
		t.Errorf("Expected 0 devices initially, got %d", len(cfg.GetDevices()))
	}

	// 1. Save devices with both attendance and help watermarks
	testDevices := []model.DeviceItem{
		{
			ID:                     "dev-1",
			Name:                   "U1-IN-BB-01",
			IsOT:                   false,
			Enabled:                true,
			AttendanceLastSyncTime: "2026-09-30T10:00:00+07:00",
			HelpLastSyncTime:       "2026-09-30T08:00:00+07:00",
		},
		{
			ID:                     "dev-2",
			Name:                   "U1-OT-BB-01",
			IsOT:                   true,
			Enabled:                true,
			AttendanceLastSyncTime: "2026-09-30T10:15:00+07:00",
			HelpLastSyncTime:       "2026-09-30T09:00:00+07:00",
		},
		{
			ID:                     "dev-3",
			Name:                   "U1-IN-BB-02",
			IsOT:                   false,
			Enabled:                false, // disabled
			AttendanceLastSyncTime: "2026-09-20T10:00:00+07:00",
			HelpLastSyncTime:       "2026-09-20T08:00:00+07:00",
		},
	}

	if err := cfg.SaveDevices(testDevices); err != nil {
		t.Fatalf("SaveDevices error: %v", err)
	}

	// Verify devices.json was written
	devFileBytes, err := os.ReadFile(devPath)
	if err != nil {
		t.Fatalf("devices.json not found: %v", err)
	}

	var savedDevs []model.DeviceItem
	if err := json.Unmarshal(devFileBytes, &savedDevs); err != nil {
		t.Fatalf("failed to unmarshal devices.json: %v", err)
	}

	if len(savedDevs) != 3 {
		t.Fatalf("expected 3 devices in devices.json, got %d", len(savedDevs))
	}
	if !savedDevs[1].IsOT {
		t.Errorf("expected dev-2 to be OT")
	}

	// Verify earliest attendance sync time across enabled devices (dev-1 vs dev-2) -> 10:00:00
	attEarliest, ok := cfg.GetEarliestDeviceSyncTime("attendance")
	if !ok {
		t.Fatalf("expected ok=true from GetEarliestDeviceSyncTime(attendance)")
	}
	expectedAtt, _ := time.Parse(time.RFC3339, "2026-09-30T10:00:00+07:00")
	if !attEarliest.Equal(expectedAtt) {
		t.Errorf("expected earliest attendance sync time %v, got %v", expectedAtt, attEarliest)
	}

	// Verify earliest help sync time across enabled devices (dev-1 vs dev-2) -> 08:00:00
	helpEarliest, ok := cfg.GetEarliestDeviceSyncTime("help")
	if !ok {
		t.Fatalf("expected ok=true from GetEarliestDeviceSyncTime(help)")
	}
	expectedHelp, _ := time.Parse(time.RFC3339, "2026-09-30T08:00:00+07:00")
	if !helpEarliest.Equal(expectedHelp) {
		t.Errorf("expected earliest help sync time %v, got %v", expectedHelp, helpEarliest)
	}

	// 2. Test Reset Device Watermark for Attendance only
	if err := cfg.ResetDeviceSyncTime("dev-1", "attendance", nil); err != nil {
		t.Fatalf("ResetDeviceSyncTime error: %v", err)
	}
	devsAfterReset := cfg.GetDevices()
	if devsAfterReset[0].AttendanceLastSyncTime != "" {
		t.Errorf("expected dev-1 attendanceLastSyncTime to be empty after reset, got %s", devsAfterReset[0].AttendanceLastSyncTime)
	}
	if devsAfterReset[0].HelpLastSyncTime == "" {
		t.Errorf("expected dev-1 helpLastSyncTime to be preserved, got empty")
	}

	// 3. Test Reset all watermarks
	if err := cfg.ResetDeviceSyncTime("dev-1", "all", nil); err != nil {
		t.Fatalf("ResetDeviceSyncTime all error: %v", err)
	}
	devsAfterResetAll := cfg.GetDevices()
	if devsAfterResetAll[0].HelpLastSyncTime != "" {
		t.Errorf("expected dev-1 helpLastSyncTime to be empty after reset all, got %s", devsAfterResetAll[0].HelpLastSyncTime)
	}

	// 4. Test reloading config from disk
	cfgReloaded, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig reload error: %v", err)
	}
	if len(cfgReloaded.GetDevices()) != 3 {
		t.Errorf("expected 3 devices after reload, got %d", len(cfgReloaded.GetDevices()))
	}
}

func TestInitialLookbackAndLimits(t *testing.T) {
	cfg := &Config{
		Jobs: JobsConfig{
			Attendance: AttendanceJobConfig{
				InitialLookbackHours: 24,
				MaxLookbackDays:      14,
			},
			Help: HelpJobConfig{
				InitialLookbackHours: 0, // start of day
				MaxLookbackDays:      0, // default 7 days
			},
		},
	}

	// 1. Attendance initial sync (24h ago)
	now := time.Now()
	attInitial := cfg.GetInitialSyncTime("attendance")
	diffAtt := now.Sub(attInitial)
	if diffAtt < 23*time.Hour || diffAtt > 25*time.Hour {
		t.Errorf("expected ~24 hours difference, got %v", diffAtt)
	}

	// 2. Help initial sync (00:00:00 of today)
	helpInitial := cfg.GetInitialSyncTime("help")
	expectedHelpStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	if !helpInitial.Equal(expectedHelpStart) {
		t.Errorf("expected start of day %v, got %v", expectedHelpStart, helpInitial)
	}

	// 3. Max lookback days check
	if cfg.GetMaxLookbackDays("attendance") != 14 {
		t.Errorf("expected 14 max lookback days for attendance, got %d", cfg.GetMaxLookbackDays("attendance"))
	}
	if cfg.GetMaxLookbackDays("help") != 7 { // default 7
		t.Errorf("expected 7 default max lookback days for help, got %d", cfg.GetMaxLookbackDays("help"))
	}
}

