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
				MaxLookbackDays: 14,
			},
			Help: HelpJobConfig{
				MaxLookbackDays: 0, // default 7 days
			},
		},
	}

	// 1. Attendance initial sync (14 days ago)
	now := time.Now()
	attInitial := cfg.GetInitialSyncTime("attendance")
	expectedAtt := now.AddDate(0, 0, -14)
	if attInitial.Day() != expectedAtt.Day() || attInitial.Month() != expectedAtt.Month() {
		t.Errorf("expected 14 days ago %v, got %v", expectedAtt, attInitial)
	}

	// 2. Help initial sync (default 7 days ago)
	helpInitial := cfg.GetInitialSyncTime("help")
	expectedHelp := now.AddDate(0, 0, -7)
	if helpInitial.Day() != expectedHelp.Day() || helpInitial.Month() != expectedHelp.Month() {
		t.Errorf("expected 7 days ago %v, got %v", expectedHelp, helpInitial)
	}

	// 3. Max lookback days check
	if cfg.GetMaxLookbackDays("attendance") != 14 {
		t.Errorf("expected 14 max lookback days for attendance, got %d", cfg.GetMaxLookbackDays("attendance"))
	}
	if cfg.GetMaxLookbackDays("help") != 7 { // default 7
		t.Errorf("expected 7 default max lookback days for help, got %d", cfg.GetMaxLookbackDays("help"))
	}
}

func TestUpdateDevicesLogBasedWatermark(t *testing.T) {
	tempDir := t.TempDir()
	devPath := filepath.Join(tempDir, "devices.json")

	cfg := &Config{
		DevicesFilePath: devPath,
		Devices: []model.DeviceItem{
			{
				ID:                     "dev-online",
				Name:                   "Online Gate",
				Enabled:                true,
				AttendanceLastSyncTime: "2026-10-31T08:00:00+07:00",
				HelpLastSyncTime:       "2026-10-31T08:00:00+07:00",
			},
			{
				ID:                     "dev-offline",
				Name:                   "Offline Gate",
				Enabled:                true,
				AttendanceLastSyncTime: "2026-10-30T17:00:00+07:00",
				HelpLastSyncTime:       "2026-10-30T17:00:00+07:00",
			},
		},
	}

	checkTime, _ := time.Parse(time.RFC3339, "2026-11-01T10:00:00+07:00")
	logTimeOnline, _ := time.Parse(time.RFC3339, "2026-11-01T09:15:30+07:00")

	// deviceMaxTimes only contains records for "dev-online", "dev-offline" has NO records (offline)
	deviceMaxTimes := map[string]time.Time{
		"dev-online": logTimeOnline,
	}

	// 1. Update Attendance Watermark
	if err := cfg.UpdateDevicesAttendanceSyncTime(deviceMaxTimes, checkTime); err != nil {
		t.Fatalf("UpdateDevicesAttendanceSyncTime error: %v", err)
	}

	devs := cfg.GetDevices()
	// dev-online should advance to logTimeOnline
	if devs[0].AttendanceLastSyncTime != logTimeOnline.Format(time.RFC3339) {
		t.Errorf("expected dev-online attendance watermark %s, got %s", logTimeOnline.Format(time.RFC3339), devs[0].AttendanceLastSyncTime)
	}
	// dev-offline MUST remain at its original watermark 2026-10-30T17:00:00+07:00
	if devs[1].AttendanceLastSyncTime != "2026-10-30T17:00:00+07:00" {
		t.Errorf("expected dev-offline attendance watermark to stay 2026-10-30T17:00:00+07:00, but got %s", devs[1].AttendanceLastSyncTime)
	}
	// Both devices should have LastCheckTime recorded
	if devs[0].LastCheckTime != checkTime.Format(time.RFC3339) || devs[1].LastCheckTime != checkTime.Format(time.RFC3339) {
		t.Errorf("expected LastCheckTime to be %s", checkTime.Format(time.RFC3339))
	}

	// 2. Update Help Watermark
	helpLogTimeOnline, _ := time.Parse(time.RFC3339, "2026-11-01T09:20:00+07:00")
	helpDeviceMaxTimes := map[string]time.Time{
		"dev-online": helpLogTimeOnline,
	}
	if err := cfg.UpdateDevicesHelpSyncTime(helpDeviceMaxTimes, checkTime); err != nil {
		t.Fatalf("UpdateDevicesHelpSyncTime error: %v", err)
	}

	devsAfterHelp := cfg.GetDevices()
	if devsAfterHelp[0].HelpLastSyncTime != helpLogTimeOnline.Format(time.RFC3339) {
		t.Errorf("expected dev-online help watermark %s, got %s", helpLogTimeOnline.Format(time.RFC3339), devsAfterHelp[0].HelpLastSyncTime)
	}
	if devsAfterHelp[1].HelpLastSyncTime != "2026-10-30T17:00:00+07:00" {
		t.Errorf("expected dev-offline help watermark to stay 2026-10-30T17:00:00+07:00, but got %s", devsAfterHelp[1].HelpLastSyncTime)
	}
}

