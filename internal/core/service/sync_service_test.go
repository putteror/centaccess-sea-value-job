package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
)

func TestIsOTDevice(t *testing.T) {
	targetNames := []string{"OT", "Gate OT", "Overtime"}
	targetIDs := []string{"dev_ot_01", "DEV-OT-99"}
	targetCodes := []string{"CODE_OT_01", "GATE_OT"}

	tests := []struct {
		name       string
		deviceName string
		deviceID   string
		deviceCode string
		expected   bool
	}{
		{
			name:       "Exact OT device name",
			deviceName: "Gate OT",
			deviceID:   "dev_123",
			expected:   true,
		},
		{
			name:       "Substring OT device name case-insensitive",
			deviceName: "Turnstile gate ot north",
			deviceID:   "dev_456",
			expected:   true,
		},
		{
			name:       "Exact OT device ID",
			deviceID:   "dev_ot_01",
			deviceName: "General Gate",
			expected:   true,
		},
		{
			name:       "Exact OT device ID case-insensitive",
			deviceID:   "DEV_OT_01",
			deviceName: "General Gate",
			expected:   true,
		},
		{
			name:       "Exact OT device Code",
			deviceName: "General Gate",
			deviceID:   "dev_unknown",
			deviceCode: "CODE_OT_01",
			expected:   true,
		},
		{
			name:       "Exact OT device Code case-insensitive",
			deviceName: "General Gate",
			deviceID:   "dev_unknown",
			deviceCode: "code_ot_01",
			expected:   true,
		},
		{
			name:       "Standard non-OT attendance turnstile",
			deviceName: "Main Entrance Turnstile #1",
			deviceID:   "dev_standard_01",
			deviceCode: "ENT_01",
			expected:   false,
		},
		{
			name:       "Cafeteria scanner",
			deviceName: "Cafeteria Gate In",
			deviceID:   "dev_canteen_02",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isOTDevice(tt.deviceName, tt.deviceID, tt.deviceCode, targetNames, targetIDs, targetCodes)
			if got != tt.expected {
				t.Errorf("isOTDevice(%q, %q, %q) = %v; want %v", tt.deviceName, tt.deviceID, tt.deviceCode, got, tt.expected)
			}
		})
	}
}

// MockCentAccessClient implements CentAccessAPIPort for testing
type mockCentAccessClient struct {
	records []model.AccessRecord
}

func (m *mockCentAccessClient) Login(ctx context.Context, username, password string) (*model.LoginResponseData, error) {
	return &model.LoginResponseData{Token: "test_token"}, nil
}

func (m *mockCentAccessClient) GetAuthToken(ctx context.Context) (string, error) {
	return "Bearer test_token", nil
}

func (m *mockCentAccessClient) ForceRefreshToken(ctx context.Context) (string, error) {
	return "Bearer test_token_new", nil
}

func (m *mockCentAccessClient) FetchAccessRecords(ctx context.Context, start, end time.Time) ([]model.AccessRecord, error) {
	return m.records, nil
}

func (m *mockCentAccessClient) FetchDevices(ctx context.Context) ([]model.DeviceItem, error) {
	return []model.DeviceItem{
		{ID: "DEV_MAIN_01", Name: "Main Entrance Door"},
		{ID: "DEV_456", Name: "Gate OT South"},
	}, nil
}

func (m *mockCentAccessClient) FetchHelpRecords(ctx context.Context, start, end time.Time) ([]model.HelpRecord, error) {
	return nil, nil
}

func (m *mockCentAccessClient) UploadPeopleExcel(ctx context.Context, filePath string, personType string) (*model.ImportResult, error) {
	return &model.ImportResult{SuccessCount: 1}, nil
}

func (m *mockCentAccessClient) ImportPeople(ctx context.Context, people []model.PeopleImportRow) (*model.ImportResult, error) {
	return &model.ImportResult{SuccessCount: len(people)}, nil
}

// MockStorage implements StoragePort for testing
type mockStorage struct {
	writtenAttendance []model.AccessRecord
	writtenOvertime   []model.AccessRecord
}

func (m *mockStorage) WriteAttendanceRecords(ctx context.Context, records []model.AccessRecord, date time.Time) (string, error) {
	m.writtenAttendance = append(m.writtenAttendance, records...)
	return "data/attendance/test.txt", nil
}

func (m *mockStorage) WriteOvertimeRecords(ctx context.Context, records []model.AccessRecord, date time.Time) (string, error) {
	m.writtenOvertime = append(m.writtenOvertime, records...)
	return "data/overtime/test.txt", nil
}

func (m *mockStorage) WriteHelpRecords(ctx context.Context, records []model.HelpRecord, date time.Time) (string, error) {
	return "data/help/test.txt", nil
}

func TestSyncAttendanceSingleFetchSplit(t *testing.T) {
	cfg := &config.Config{
		Jobs: config.JobsConfig{
			Attendance: config.AttendanceJobConfig{
				ExcludeDeviceNames: []string{"OT", "Gate OT"},
				ExcludeDeviceIDs:   []string{"DEV_OT_01"},
			},
			Overtime: config.OvertimeJobConfig{
				TargetDeviceNames: []string{"OT", "Gate OT"},
				TargetDeviceIDs:   []string{"DEV_OT_01"},
			},
		},
	}

	rawRecords := []model.AccessRecord{
		{
			PersonCode:              "EMP001",
			AccessControlDeviceName: "Main Entrance Door",
			AccessControlDeviceID:   "DEV_MAIN_01",
			RecordTime:              time.Now(),
		},
		{
			PersonCode:              "EMP002",
			AccessControlDeviceName: "Gate OT South",
			AccessControlDeviceID:   "DEV_456",
			RecordTime:              time.Now(),
		},
		{
			PersonCode:              "EMP003",
			AccessControlDeviceName: "Back Door",
			AccessControlDeviceID:   "DEV_OT_01",
			RecordTime:              time.Now(),
		},
		{
			PersonCode:              "EMP004",
			AccessControlDeviceName: "Lobby Turnstile",
			AccessControlDeviceID:   "DEV_LOBBY",
			RecordTime:              time.Now(),
		},
	}

	mockClient := &mockCentAccessClient{records: rawRecords}
	mockStore := &mockStorage{}

	service := NewAttendanceSyncService(cfg, mockClient, mockStore)

	ctx := context.Background()
	now := time.Now()
	err := service.SyncAttendance(ctx, now, now, true)
	if err != nil {
		t.Fatalf("SyncAttendance returned error: %v", err)
	}

	// Attendance should have 2 non-OT records (EMP001, EMP004)
	if len(mockStore.writtenAttendance) != 2 {
		t.Errorf("expected 2 attendance records, got %d", len(mockStore.writtenAttendance))
	}

	// Overtime should have 2 OT records (EMP002, EMP003)
	if len(mockStore.writtenOvertime) != 2 {
		t.Errorf("expected 2 overtime records, got %d", len(mockStore.writtenOvertime))
	}
}

func TestSyncAttendanceOfflineDevicePreservesWatermark(t *testing.T) {
	tempDir := t.TempDir()
	devPath := filepath.Join(tempDir, "devices.json")

	cfg := &config.Config{
		DevicesFilePath: devPath,
		Jobs: config.JobsConfig{
			Attendance: config.AttendanceJobConfig{
				ExportFolder: "attendance",
				Format:       "txt",
			},
		},
		Devices: []model.DeviceItem{
			{
				ID:                     "DEV_ONLINE",
				Name:                   "Online Gate",
				Enabled:                true,
				AttendanceLastSyncTime: "2026-10-31T08:00:00+07:00",
			},
			{
				ID:                     "DEV_OFFLINE",
				Name:                   "Offline Gate",
				Enabled:                true,
				AttendanceLastSyncTime: "2026-10-30T17:00:00+07:00",
			},
		},
	}

	recordTime, _ := time.Parse(time.RFC3339, "2026-11-01T08:30:00+07:00")
	// Only DEV_ONLINE has records in the fetched batch
	rawRecords := []model.AccessRecord{
		{
			PersonCode:              "EMP001",
			AccessControlDeviceName: "Online Gate",
			AccessControlDeviceID:   "DEV_ONLINE",
			RecordTime:              recordTime,
		},
	}

	mockClient := &mockCentAccessClient{records: rawRecords}
	mockStore := &mockStorage{}

	attService := NewAttendanceSyncService(cfg, mockClient, mockStore)

	ctx := context.Background()
	checkTime, _ := time.Parse(time.RFC3339, "2026-11-01T10:00:00+07:00")

	if err := attService.SyncAttendance(ctx, recordTime.Add(-1*time.Hour), checkTime, true); err != nil {
		t.Fatalf("SyncAttendance error: %v", err)
	}

	devs := cfg.GetDevices()
	// DEV_ONLINE should advance to recordTime
	if devs[0].AttendanceLastSyncTime != recordTime.Format(time.RFC3339) {
		t.Errorf("expected DEV_ONLINE watermark to be %s, got %s", recordTime.Format(time.RFC3339), devs[0].AttendanceLastSyncTime)
	}
	// DEV_OFFLINE MUST preserve 2026-10-30T17:00:00+07:00
	if devs[1].AttendanceLastSyncTime != "2026-10-30T17:00:00+07:00" {
		t.Errorf("expected DEV_OFFLINE watermark to remain 2026-10-30T17:00:00+07:00, got %s", devs[1].AttendanceLastSyncTime)
	}
}

func TestSyncAttendanceDuplicateRecordAtWatermarkTime(t *testing.T) {
	tempDir := t.TempDir()
	devPath := filepath.Join(tempDir, "devices.json")

	cfg := &config.Config{
		DevicesFilePath: devPath,
		Jobs: config.JobsConfig{
			Attendance: config.AttendanceJobConfig{
				ExportFolder: "attendance",
				Format:       "txt",
			},
			Overtime: config.OvertimeJobConfig{
				TargetDeviceNames: []string{"E88"},
			},
		},
		Devices: []model.DeviceItem{
			{
				ID:                     "5a2e0d16-b743-4b88-b2f7-af7c89d75411",
				Name:                   "Shenxing_E88",
				Enabled:                true,
				AttendanceLastSyncTime: "2026-10-24T15:48:09Z",
			},
		},
	}

	// Case A: Exact same second with subseconds .345
	recTimeWithSubsecond, _ := time.Parse(time.RFC3339Nano, "2026-10-24T15:48:09.345Z")

	rawRecords := []model.AccessRecord{
		{
			PersonCode:              "EMP001",
			AccessControlDeviceName: "E88",
			AccessControlDeviceID:   "5a2e0d16-b743-4b88-b2f7-af7c89d75411",
			RecordTime:              recTimeWithSubsecond,
		},
	}

	mockClient := &mockCentAccessClient{records: rawRecords}
	mockStore := &mockStorage{}

	attService := NewAttendanceSyncService(cfg, mockClient, mockStore)

	ctx := context.Background()
	checkTime := time.Now()

	start, _ := time.Parse(time.RFC3339, "2026-10-24T15:48:09Z")
	if err := attService.SyncAttendance(ctx, start, checkTime, true); err != nil {
		t.Fatalf("SyncAttendance error: %v", err)
	}

	t.Logf("Written attendance: %d, written overtime: %d", len(mockStore.writtenAttendance), len(mockStore.writtenOvertime))
	if len(mockStore.writtenOvertime) > 0 {
		t.Errorf("BUG REPRODUCED! Record at 22:48:09 was written again! Count: %d", len(mockStore.writtenOvertime))
	}
}

func TestSyncHelpRecords(t *testing.T) {
	cfg := &config.Config{
		Jobs: config.JobsConfig{
			Help: config.HelpJobConfig{
				ExportFolder: "help",
				Format:       "txt",
			},
		},
	}

	inTime := time.Now().Add(-2 * time.Hour)
	outTime := time.Now().Add(-1 * time.Hour)
	helpRecords := []model.HelpRecord{
		{
			PersonCode:              "6103510013",
			CheckInTime:             &inTime,
			CheckOutTime:            &outTime,
			Status:                  "CHECKED_OUT",
			AccessControlDeviceName: "U1-IN-BB-01",
			AccessControlDeviceID:   "DEV_HELP_01",
			HelpDepartment:          "0000000000",
		},
	}

	mockClient := &mockCentAccessClient{}
	mockClientWithHelp := &mockCentAccessClientWithHelp{helpRecords: helpRecords}
	mockStore := &mockStorage{}

	_ = mockClient
	service := NewHelpSyncService(cfg, mockClientWithHelp, mockStore)

	ctx := context.Background()
	now := time.Now()
	err := service.SyncHelp(ctx, now, now, true)
	if err != nil {
		t.Fatalf("SyncHelp returned error: %v", err)
	}
}

type mockCentAccessClientWithHelp struct {
	mockCentAccessClient
	helpRecords []model.HelpRecord
}

func (m *mockCentAccessClientWithHelp) FetchHelpRecords(ctx context.Context, start, end time.Time) ([]model.HelpRecord, error) {
	return m.helpRecords, nil
}

func TestDeviceService(t *testing.T) {
	cfg := &config.Config{
		Jobs: config.JobsConfig{
			Overtime: config.OvertimeJobConfig{
				TargetDeviceNames: []string{"Gate OT"},
				TargetDeviceIDs:   []string{"DEV_456"},
			},
		},
	}

	mockClient := &mockCentAccessClient{}
	deviceService := NewDeviceService(cfg, mockClient)

	// Test IsOTDevice
	if !deviceService.IsOTDevice("Gate OT South", "DEV_456") {
		t.Errorf("expected Gate OT South to be OT device")
	}
	if deviceService.IsOTDevice("Main Entrance Door", "DEV_MAIN_01") {
		t.Errorf("expected Main Entrance Door to be non-OT device")
	}
}

func TestExportMethods(t *testing.T) {
	cfg := &config.Config{
		Jobs: config.JobsConfig{
			Overtime: config.OvertimeJobConfig{
				TargetDeviceNames: []string{"OT"},
			},
		},
	}

	now := time.Now()
	records := []model.AccessRecord{
		{
			PersonCode:              "EMP001",
			AccessControlDeviceName: "Main Entrance Door",
			AccessControlDeviceID:   "DEV_MAIN_01",
			RecordTime:              now,
		},
		{
			PersonCode:              "EMP002",
			AccessControlDeviceName: "Gate OT South",
			AccessControlDeviceID:   "DEV_OT_02",
			RecordTime:              now,
		},
	}

	helpInTime := now.Add(-2 * time.Hour)
	helpOutTime := now
	helpRecords := []model.HelpRecord{
		{
			PersonCode:              "HELP001",
			CheckInTime:             &helpInTime,
			CheckOutTime:            &helpOutTime,
			Status:                  "CHECKED_OUT",
			AccessControlDeviceName: "Main Entrance Door",
			AccessControlDeviceID:   "DEV_MAIN_01",
			HelpDepartment:          "HR",
		},
	}

	mockClient := &mockCentAccessClient{records: records}
	mockClientWithHelp := &mockCentAccessClientWithHelp{helpRecords: helpRecords}
	mockStore := &mockStorage{}

	attendanceService := NewAttendanceSyncService(cfg, mockClient, mockStore)
	helpService := NewHelpSyncService(cfg, mockClientWithHelp, mockStore)

	ctx := context.Background()

	// 1. Export Attendance TXT
	attRes, err := attendanceService.ExportAttendanceTXT(ctx, now, now, nil)
	if err != nil {
		t.Fatalf("ExportAttendanceTXT error: %v", err)
	}
	if attRes.RecordCount != 1 {
		t.Errorf("expected 1 attendance record in export, got %d", attRes.RecordCount)
	}
	if !strings.Contains(string(attRes.Content), "EMP001") {
		t.Errorf("expected content to contain EMP001, got: %s", string(attRes.Content))
	}

	// 2. Export Overtime TXT
	otRes, err := attendanceService.ExportOvertimeTXT(ctx, now, now, nil)
	if err != nil {
		t.Fatalf("ExportOvertimeTXT error: %v", err)
	}
	if otRes.RecordCount != 1 {
		t.Errorf("expected 1 overtime record in export, got %d", otRes.RecordCount)
	}
	if !strings.Contains(string(otRes.Content), "EMP002") {
		t.Errorf("expected content to contain EMP002, got: %s", string(otRes.Content))
	}

	// 3. Export Help TXT
	helpRes, err := helpService.ExportHelpTXT(ctx, now, now, nil)
	if err != nil {
		t.Fatalf("ExportHelpTXT error: %v", err)
	}
	if helpRes.RecordCount != 1 {
		t.Errorf("expected 1 help record in export, got %d", helpRes.RecordCount)
	}
	if !strings.Contains(string(helpRes.Content), "HELP001") {
		t.Errorf("expected content to contain HELP001, got: %s", string(helpRes.Content))
	}
}

func TestSyncAttendanceFiltersSuccessOnly(t *testing.T) {
	cfg := &config.Config{
		Jobs: config.JobsConfig{
			Attendance: config.AttendanceJobConfig{},
			Overtime:   config.OvertimeJobConfig{},
		},
	}

	rawRecords := []model.AccessRecord{
		{
			PersonCode:              "EMP_SUCCESS_1",
			AccessControlDeviceName: "Main Entrance",
			AccessControlDeviceID:   "DEV_1",
			Result:                  "SUCCESS",
			RecordTime:              time.Now(),
		},
		{
			PersonCode:              "EMP_FAILED_1",
			AccessControlDeviceName: "Main Entrance",
			AccessControlDeviceID:   "DEV_1",
			Result:                  "FAILED",
			RecordTime:              time.Now(),
		},
		{
			PersonCode:              "EMP_DENIED_1",
			AccessControlDeviceName: "Main Entrance",
			AccessControlDeviceID:   "DEV_1",
			Result:                  "DENIED",
			RecordTime:              time.Now(),
		},
		{
			PersonCode:              "EMP_SUCCESS_2",
			AccessControlDeviceName: "Main Entrance",
			AccessControlDeviceID:   "DEV_1",
			Result:                  "success",
			RecordTime:              time.Now(),
		},
	}

	mockClient := &mockCentAccessClient{records: rawRecords}
	mockStore := &mockStorage{}

	service := NewAttendanceSyncService(cfg, mockClient, mockStore)

	ctx := context.Background()
	now := time.Now()
	err := service.SyncAttendance(ctx, now, now, true)
	if err != nil {
		t.Fatalf("SyncAttendance returned error: %v", err)
	}

	// Only EMP_SUCCESS_1 and EMP_SUCCESS_2 should be written
	if len(mockStore.writtenAttendance) != 2 {
		t.Fatalf("expected 2 successful attendance records, got %d", len(mockStore.writtenAttendance))
	}
	if mockStore.writtenAttendance[0].PersonCode != "EMP_SUCCESS_1" || mockStore.writtenAttendance[1].PersonCode != "EMP_SUCCESS_2" {
		t.Errorf("unexpected written records: %+v", mockStore.writtenAttendance)
	}

	// Test ExportAttendanceTXT as well
	exportRes, err := service.ExportAttendanceTXT(ctx, now, now, nil)
	if err != nil {
		t.Fatalf("ExportAttendanceTXT returned error: %v", err)
	}
	if exportRes.RecordCount != 2 {
		t.Errorf("expected export record count 2, got %d", exportRes.RecordCount)
	}
	if strings.Contains(string(exportRes.Content), "EMP_FAILED_1") || strings.Contains(string(exportRes.Content), "EMP_DENIED_1") {
		t.Errorf("export content should not contain failed records: %s", string(exportRes.Content))
	}
}

func TestSyncHelpFiltersCheckInAndCheckOut(t *testing.T) {
	cfg := &config.Config{
		Jobs: config.JobsConfig{
			Help: config.HelpJobConfig{
				ExportFolder: "help",
			},
		},
	}

	t1 := time.Now().Add(-3 * time.Hour)
	t2 := time.Now().Add(-2 * time.Hour)
	t3 := time.Now().Add(-1 * time.Hour)

	mockClient := &mockCentAccessClientWithHelp{
		helpRecords: []model.HelpRecord{
			{
				PersonCode:              "HELP_IN_ONLY",
				CheckInTime:             &t1,
				CheckOutTime:            nil,
				Status:                  "CHECKED_IN",
				AccessControlDeviceName: "Gate Help",
				AccessControlDeviceID:   "DEV_HELP",
			},
			{
				PersonCode:              "HELP_NO_IN",
				CheckInTime:             nil,
				CheckOutTime:            &t2,
				Status:                  "CHECKED_OUT",
				AccessControlDeviceName: "Gate Help",
				AccessControlDeviceID:   "DEV_HELP",
			},
			{
				PersonCode:              "HELP_COMPLETED_1",
				CheckInTime:             &t1,
				CheckOutTime:            &t2,
				Status:                  "CHECKED_OUT",
				AccessControlDeviceName: "Gate Help",
				AccessControlDeviceID:   "DEV_HELP",
			},
			{
				PersonCode:              "HELP_COMPLETED_2",
				CheckInTime:             &t2,
				CheckOutTime:            &t3,
				Status:                  "AUTO_CLOSED",
				AccessControlDeviceName: "Gate Help",
				AccessControlDeviceID:   "DEV_HELP",
			},
		},
	}

	mockStore := &mockStorage{}
	service := NewHelpSyncService(cfg, mockClient, mockStore)
	ctx := context.Background()

	// 1. Test ExportHelpTXT only includes HELP_COMPLETED_1 and HELP_COMPLETED_2
	exportRes, err := service.ExportHelpTXT(ctx, time.Time{}, time.Time{}, nil)
	if err != nil {
		t.Fatalf("ExportHelpTXT error: %v", err)
	}
	if exportRes.RecordCount != 2 {
		t.Fatalf("expected 2 complete help records in export, got %d", exportRes.RecordCount)
	}

	contentStr := string(exportRes.Content)
	if strings.Contains(contentStr, "HELP_IN_ONLY") {
		t.Errorf("export content should not contain HELP_IN_ONLY: %s", contentStr)
	}
	if strings.Contains(contentStr, "HELP_NO_IN") {
		t.Errorf("export content should not contain HELP_NO_IN: %s", contentStr)
	}
	if !strings.Contains(contentStr, "HELP_COMPLETED_1") || !strings.Contains(contentStr, "HELP_COMPLETED_2") {
		t.Errorf("export content missing completed records: %s", contentStr)
	}
}



