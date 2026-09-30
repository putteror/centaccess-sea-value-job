package storage

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
)

func TestStorageFileFormatting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "seavalue_storage_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		ExportBaseDir: tmpDir,
		Jobs: config.JobsConfig{
			Attendance: config.AttendanceJobConfig{ExportFolder: "attendance"},
			Overtime:   config.OvertimeJobConfig{ExportFolder: "overtime"},
			Help:       config.HelpJobConfig{ExportFolder: "help"},
		},
	}
	adapter := NewFileStorageAdapter(cfg)
	ctx := context.Background()
	testDate := time.Date(2025, 6, 23, 0, 0, 0, 0, time.UTC)

	t.Run("Attendance (INOUT) Formatting with Bangkok Timezone", func(t *testing.T) {
		// UTC 03:18:00 -> Bangkok (UTC+7) is 10:18:00
		recTime1 := time.Date(2025, 6, 23, 3, 18, 0, 0, time.UTC)
		// UTC 12:18:00 -> Bangkok (UTC+7) is 19:18:00
		recTime2 := time.Date(2025, 6, 23, 12, 18, 0, 0, time.UTC)

		records := []model.AccessRecord{
			{
				PersonCode:              "6103510013",
				RecordTime:              recTime1,
				AccessControlDeviceName: "U1-IN-BB-01",
			},
			{
				PersonCode:              "6103510013",
				RecordTime:              recTime2,
				AccessControlDeviceName: "U1-IN-BB-01",
			},
		}

		filePath, err := adapter.WriteAttendanceRecords(ctx, records, testDate)
		if err != nil {
			t.Fatalf("WriteAttendanceRecords error: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}

		expected := "6103510013 20250623 101800 U1-IN-BB-01\n6103510013 20250623 191800 U1-IN-BB-01\n"
		if string(content) != expected {
			t.Errorf("Attendance content mismatch.\nGot:\n%s\nWant:\n%s", string(content), expected)
		}
	})

	t.Run("Overtime (OT) Formatting with Bangkok Timezone", func(t *testing.T) {
		// UTC 03:18:00 -> Bangkok (UTC+7) is 10:18:00
		recTime1 := time.Date(2025, 6, 23, 3, 18, 0, 0, time.UTC)
		// UTC 12:18:00 -> Bangkok (UTC+7) is 19:18:00
		recTime2 := time.Date(2025, 6, 23, 12, 18, 0, 0, time.UTC)

		records := []model.AccessRecord{
			{
				PersonCode:              "6103510013",
				RecordTime:              recTime1,
				AccessControlDeviceName: "U1-OT-BB-01",
			},
			{
				PersonCode:              "6103510013",
				RecordTime:              recTime2,
				AccessControlDeviceName: "U1-OT-BB-01",
			},
		}

		filePath, err := adapter.WriteOvertimeRecords(ctx, records, testDate)
		if err != nil {
			t.Fatalf("WriteOvertimeRecords error: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}

		expected := "6103510013 20250623 101800 U1-OT-BB-01\n6103510013 20250623 191800 U1-OT-BB-01\n"
		if string(content) != expected {
			t.Errorf("Overtime content mismatch.\nGot:\n%s\nWant:\n%s", string(content), expected)
		}
	})

	t.Run("Help Formatting with Bangkok Timezone", func(t *testing.T) {
		// UTC 03:14:00 -> Bangkok (UTC+7) is 10:14:00
		t1 := time.Date(2025, 6, 23, 3, 14, 0, 0, time.UTC)
		// UTC 03:20:00 -> Bangkok (UTC+7) is 10:20:00
		t2 := time.Date(2025, 6, 23, 3, 20, 0, 0, time.UTC)

		records := []model.HelpRecord{
			{
				PersonCode:              "6103510013",
				CheckInTime:             &t1,
				CheckOutTime:            &t1,
				AccessControlDeviceName: "U1-IN-BB-01",
				HelpDepartment:          "0000000000",
			},
			{
				PersonCode:              "6103510013",
				CheckInTime:             &t2,
				CheckOutTime:            &t2,
				AccessControlDeviceName: "U1-IN-BB-01",
			},
		}

		filePath, err := adapter.WriteHelpRecords(ctx, records, testDate)
		if err != nil {
			t.Fatalf("WriteHelpRecords error: %v", err)
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("ReadFile error: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(string(content)), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 lines, got %d", len(lines))
		}

		expected1 := "6103510013 20250623 101400 20250623 101400 U1-IN-BB-01 0000000000"
		expected2 := "6103510013 20250623 102000 20250623 102000 U1-IN-BB-01 0000000000"

		if lines[0] != expected1 {
			t.Errorf("Line 1 mismatch.\nGot:  %s\nWant: %s", lines[0], expected1)
		}
		if lines[1] != expected2 {
			t.Errorf("Line 2 mismatch.\nGot:  %s\nWant: %s", lines[1], expected2)
		}
	})
}
