package model_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

func TestHelpRecordFlexibleUnmarshal(t *testing.T) {
	// Case 1: Standard RFC3339
	json1 := `{
		"id": "help-1",
		"personCode": "EMP001",
		"checkInTime": "2026-10-24T08:30:00Z",
		"checkOutTime": "2026-10-24T17:30:00Z",
		"status": "CHECKED_OUT",
		"accessControlDeviceCode": "DEV_GATE_1",
		"accessControlDeviceId": "DEV01"
	}`
	var r1 model.HelpRecord
	if err := json.Unmarshal([]byte(json1), &r1); err != nil {
		t.Fatalf("failed to unmarshal standard json: %v", err)
	}
	if r1.CheckInTime == nil || r1.CheckOutTime == nil {
		t.Fatalf("expected parsed check-in and check-out times")
	}
	if !r1.HasCheckInAndOut() {
		t.Errorf("expected HasCheckInAndOut=true for completed record")
	}

	// Case 2: Space-separated datetime & alternative naming (checkinTime, checkoutTime, deviceCode, deviceId, employeeCode)
	json2 := `{
		"id": "help-2",
		"employeeCode": "EMP002",
		"checkinTime": "2026-10-24 08:30:00",
		"checkoutTime": "2026-10-24 17:30:00",
		"status": "CHECKED_OUT",
		"deviceCode": "ALT_CODE_88",
		"deviceId": "DEV88"
	}`
	var r2 model.HelpRecord
	if err := json.Unmarshal([]byte(json2), &r2); err != nil {
		t.Fatalf("failed to unmarshal space-separated datetime json: %v", err)
	}
	if r2.PersonCode != "EMP002" {
		t.Errorf("expected PersonCode EMP002, got %s", r2.PersonCode)
	}
	if r2.AccessControlDeviceCode != "ALT_CODE_88" {
		t.Errorf("expected DeviceCode ALT_CODE_88, got %s", r2.AccessControlDeviceCode)
	}
	if r2.AccessControlDeviceID != "DEV88" {
		t.Errorf("expected DeviceID DEV88, got %s", r2.AccessControlDeviceID)
	}
	if r2.CheckInTime == nil || r2.CheckOutTime == nil {
		t.Fatalf("expected parsed check-in and check-out times from checkinTime/checkoutTime")
	}
	if r2.CheckInTime.Hour() != 8 || r2.CheckInTime.Minute() != 30 {
		t.Errorf("unexpected check-in time: %v", r2.CheckInTime)
	}
	if !r2.HasCheckInAndOut() {
		t.Errorf("expected HasCheckInAndOut=true for r2")
	}

	// Case 3: Checked in only (in progress) -> HasCheckInAndOut should be false
	json3 := `{
		"id": "help-3",
		"personCode": "EMP003",
		"check_in_time": "2026-10-24T08:30:00Z",
		"status": "CHECKED_IN"
	}`
	var r3 model.HelpRecord
	if err := json.Unmarshal([]byte(json3), &r3); err != nil {
		t.Fatalf("failed to unmarshal in-progress record: %v", err)
	}
	if r3.HasCheckInAndOut() {
		t.Errorf("expected HasCheckInAndOut=false for CHECKED_IN status")
	}
}

func TestHelpRecordEpochTimestamps(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	milli := float64(now.UnixMilli())

	rawJSON := `{
		"id": "help-epoch",
		"checkInTime": ` + string(rune(milli)) + `,
		"status": "CHECKED_OUT"
	}`
	// Use formatted float string
	rawJSON = `{"id":"help-epoch","checkInTime":1761300000000,"status":"CHECKED_OUT"}`

	var r model.HelpRecord
	if err := json.Unmarshal([]byte(rawJSON), &r); err != nil {
		t.Fatalf("failed to unmarshal epoch json: %v", err)
	}
	if r.CheckInTime == nil {
		t.Errorf("expected parsed checkInTime from epoch millis")
	}
}

