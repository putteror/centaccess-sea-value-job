package model

import (
	"strings"
	"testing"
	"time"
)

func TestFormatAccessRecordsToTXTSorting(t *testing.T) {
	bkk := GetBangkokLocation()

	// Provide records in DESCENDING order (newest first, oldest last)
	time1 := time.Date(2026, 10, 21, 15, 0, 0, 0, bkk) // Latest
	time2 := time.Date(2026, 10, 21, 10, 0, 0, 0, bkk)
	time3 := time.Date(2026, 10, 21, 8, 0, 0, 0, bkk)  // Earliest

	records := []AccessRecord{
		{
			PersonCode:              "EMP003",
			RecordTime:              time1,
			AccessControlDeviceName: "Gate A",
		},
		{
			PersonCode:              "EMP002",
			RecordTime:              time2,
			AccessControlDeviceName: "Gate A",
		},
		{
			PersonCode:              "EMP001",
			RecordTime:              time3,
			AccessControlDeviceName: "Gate A",
		},
	}

	output := FormatAccessRecordsToTXT(records)
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Line 1 should be earliest (08:00:00)
	if !strings.Contains(lines[0], "080000") || !strings.Contains(lines[0], "EMP001") {
		t.Errorf("expected line 1 to be earliest record, got: %s", lines[0])
	}
	// Line 2 should be middle (10:00:00)
	if !strings.Contains(lines[1], "100000") || !strings.Contains(lines[1], "EMP002") {
		t.Errorf("expected line 2 to be middle record, got: %s", lines[1])
	}
	// Line 3 should be latest (15:00:00) at bottom
	if !strings.Contains(lines[2], "150000") || !strings.Contains(lines[2], "EMP003") {
		t.Errorf("expected line 3 to be latest record at bottom, got: %s", lines[2])
	}
}

func TestFormatHelpRecordsToTXTSorting(t *testing.T) {
	bkk := GetBangkokLocation()

	// Provide records in DESCENDING order (newest check-in first, oldest last)
	inTime1 := time.Date(2026, 10, 21, 21, 8, 18, 0, bkk)
	outTime1 := time.Date(2026, 10, 21, 21, 16, 33, 0, bkk)

	inTime2 := time.Date(2026, 10, 21, 15, 21, 52, 0, bkk)
	outTime2 := time.Date(2026, 10, 21, 16, 20, 11, 0, bkk)

	inTime3 := time.Date(2026, 10, 21, 8, 30, 0, 0, bkk)
	outTime3 := time.Date(2026, 10, 21, 12, 0, 0, 0, bkk)

	records := []HelpRecord{
		{
			PersonCode:              "EMP_NIGHT",
			CheckInTime:             &inTime1,
			CheckOutTime:            &outTime1,
			Status:                  "CHECKED_OUT",
			AccessControlDeviceName: "E88",
			HelpDepartment:          "C-1111",
		},
		{
			PersonCode:              "EMP_AFTERNOON",
			CheckInTime:             &inTime2,
			CheckOutTime:            &outTime2,
			Status:                  "CHECKED_OUT",
			AccessControlDeviceName: "E88",
			HelpDepartment:          "C-1111",
		},
		{
			PersonCode:              "EMP_MORNING",
			CheckInTime:             &inTime3,
			CheckOutTime:            &outTime3,
			Status:                  "CHECKED_OUT",
			AccessControlDeviceName: "E88",
			HelpDepartment:          "C-1111",
		},
	}

	output := FormatHelpRecordsToTXT(records, time.Time{})
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	// Line 1 should be earliest (08:30:00)
	if !strings.Contains(lines[0], "083000") || !strings.Contains(lines[0], "EMP_MORNING") {
		t.Errorf("expected line 1 to be earliest record, got: %s", lines[0])
	}
	// Line 2 should be afternoon (15:21:52)
	if !strings.Contains(lines[1], "152152") || !strings.Contains(lines[1], "EMP_AFTERNOON") {
		t.Errorf("expected line 2 to be middle record, got: %s", lines[1])
	}
	// Line 3 should be latest night (21:08:18) at bottom
	if !strings.Contains(lines[2], "210818") || !strings.Contains(lines[2], "EMP_NIGHT") {
		t.Errorf("expected line 3 to be latest record at bottom, got: %s", lines[2])
	}
}

func TestGenerateTXTFilename(t *testing.T) {
	bkk := GetBangkokLocation()
	testTime := time.Date(2026, 5, 14, 13, 7, 45, 0, bkk)

	tests := []struct {
		name     string
		prefix   string
		start    time.Time
		end      time.Time
		expected string
	}{
		{
			name:     "HELP uppercase",
			prefix:   "HELP",
			start:    testTime,
			end:      testTime,
			expected: "HELP_20260514_1307.txt",
		},
		{
			name:     "HELP lowercase fallback",
			prefix:   "help",
			start:    testTime,
			end:      testTime,
			expected: "HELP_20260514_1307.txt",
		},
		{
			name:     "OT uppercase",
			prefix:   "OT",
			start:    testTime,
			end:      testTime,
			expected: "OT_20260514_1307.txt",
		},
		{
			name:     "Overtime normalized to OT",
			prefix:   "overtime",
			start:    testTime,
			end:      testTime,
			expected: "OT_20260514_1307.txt",
		},
		{
			name:     "AT uppercase",
			prefix:   "AT",
			start:    testTime,
			end:      testTime,
			expected: "AT_20260514_1307.txt",
		},
		{
			name:     "Attendance normalized to AT",
			prefix:   "attendance",
			start:    testTime,
			end:      testTime,
			expected: "AT_20260514_1307.txt",
		},
		{
			name:     "Date range multi-day format",
			prefix:   "AT",
			start:    time.Date(2026, 5, 10, 8, 0, 0, 0, bkk),
			end:      time.Date(2026, 5, 14, 13, 7, 0, 0, bkk),
			expected: "AT_20260510_20260514_1307.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := GenerateTXTFilename(tt.prefix, tt.start, tt.end)
			if actual != tt.expected {
				t.Errorf("GenerateTXTFilename(%q) = %q, want %q", tt.prefix, actual, tt.expected)
			}
		})
	}
}

