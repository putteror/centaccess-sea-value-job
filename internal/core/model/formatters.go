package model

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	bangkokLocation *time.Location
	locOnce         sync.Once
)

// GetBangkokLocation returns Asia/Bangkok (UTC+7) timezone location
func GetBangkokLocation() *time.Location {
	locOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Bangkok")
		if err != nil {
			loc = time.FixedZone("Asia/Bangkok", 7*3600)
		}
		bangkokLocation = loc
	})
	return bangkokLocation
}

// FormatAccessRecordsToTXT formats a list of AccessRecord items into standard 4-column TXT format:
// person_code scan_date scan_time device_name
// Records are sorted chronologically in ascending order (earliest time first, latest time at bottom).
func FormatAccessRecordsToTXT(records []AccessRecord) string {
	bkk := GetBangkokLocation()

	sortedRecords := make([]AccessRecord, len(records))
	copy(sortedRecords, records)
	sort.SliceStable(sortedRecords, func(i, j int) bool {
		if !sortedRecords[i].RecordTime.Equal(sortedRecords[j].RecordTime) {
			return sortedRecords[i].RecordTime.Before(sortedRecords[j].RecordTime)
		}
		return sortedRecords[i].PersonCode < sortedRecords[j].PersonCode
	})

	var sb strings.Builder
	for _, r := range sortedRecords {
		personIdentifier := strings.TrimSpace(r.PersonCode)
		if personIdentifier == "" {
			personIdentifier = strings.TrimSpace(r.PersonID)
		}
		if personIdentifier == "" {
			personIdentifier = "0000000000"
		}

		recTimeBkk := r.RecordTime.In(bkk)
		scanDate := recTimeBkk.Format("20060102")
		scanTime := recTimeBkk.Format("150405")

		deviceIdentifier := strings.TrimSpace(r.AccessControlDeviceCode)
		if deviceIdentifier == "" {
			deviceIdentifier = strings.TrimSpace(r.AccessControlDeviceName)
		}
		if deviceIdentifier == "" {
			deviceIdentifier = strings.TrimSpace(r.AccessControlDeviceID)
		}
		if deviceIdentifier == "" {
			deviceIdentifier = "UNKNOWN"
		}

		sb.WriteString(fmt.Sprintf("%s %s %s %s\n", personIdentifier, scanDate, scanTime, deviceIdentifier))
	}
	return sb.String()
}

// FormatHelpRecordsToTXT formats a list of HelpRecord items into standard 7-column TXT format:
// person_code check_in_date check_in_time check_out_date check_out_time device_code/name help_department
// Records are sorted chronologically in ascending order (earliest check-in first, latest check-in at bottom).
func FormatHelpRecordsToTXT(records []HelpRecord, fallbackDate time.Time) string {
	bkk := GetBangkokLocation()
	if fallbackDate.IsZero() {
		fallbackDate = time.Now().In(bkk)
	}

	sortedRecords := make([]HelpRecord, len(records))
	copy(sortedRecords, records)
	sort.SliceStable(sortedRecords, func(i, j int) bool {
		var ti, tj time.Time
		if sortedRecords[i].CheckInTime != nil {
			ti = *sortedRecords[i].CheckInTime
		}
		if sortedRecords[j].CheckInTime != nil {
			tj = *sortedRecords[j].CheckInTime
		}
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}

		var outI, outJ time.Time
		if sortedRecords[i].CheckOutTime != nil {
			outI = *sortedRecords[i].CheckOutTime
		}
		if sortedRecords[j].CheckOutTime != nil {
			outJ = *sortedRecords[j].CheckOutTime
		}
		if !outI.Equal(outJ) {
			return outI.Before(outJ)
		}

		return sortedRecords[i].PersonCode < sortedRecords[j].PersonCode
	})

	var sb strings.Builder
	for _, r := range sortedRecords {
		if !r.HasCheckInAndOut() {
			continue
		}

		personIdentifier := strings.TrimSpace(r.PersonCode)
		if personIdentifier == "" {
			personIdentifier = strings.TrimSpace(r.PersonID)
		}
		if personIdentifier == "" {
			personIdentifier = "0000000000"
		}

		checkInDate := fallbackDate.In(bkk).Format("20060102")
		checkInTime := "000000"
		if r.CheckInTime != nil && !r.CheckInTime.IsZero() {
			inTimeBkk := r.CheckInTime.In(bkk)
			checkInDate = inTimeBkk.Format("20060102")
			checkInTime = inTimeBkk.Format("150405")
		}

		checkOutDate := checkInDate
		checkOutTime := checkInTime
		if r.CheckOutTime != nil && !r.CheckOutTime.IsZero() {
			outTimeBkk := r.CheckOutTime.In(bkk)
			checkOutDate = outTimeBkk.Format("20060102")
			checkOutTime = outTimeBkk.Format("150405")
		}

		deviceIdentifier := strings.TrimSpace(r.AccessControlDeviceCode)
		if deviceIdentifier == "" {
			deviceIdentifier = strings.TrimSpace(r.AccessControlDeviceName)
		}
		if deviceIdentifier == "" {
			deviceIdentifier = strings.TrimSpace(r.AccessControlDeviceID)
		}
		if deviceIdentifier == "" {
			deviceIdentifier = "UNKNOWN"
		}

		helpDepartment := strings.TrimSpace(r.HelpDepartment)
		if helpDepartment == "" {
			helpDepartment = strings.TrimSpace(r.HelpDepartmentCode)
		}
		if helpDepartment == "" {
			helpDepartment = strings.TrimSpace(r.DepartmentCode)
		}
		if helpDepartment == "" {
			helpDepartment = strings.TrimSpace(r.PersonDepartment)
		}
		if helpDepartment == "" {
			helpDepartment = "0000000000"
		}

		sb.WriteString(fmt.Sprintf("%s %s %s %s %s %s %s\n",
			personIdentifier,
			checkInDate,
			checkInTime,
			checkOutDate,
			checkOutTime,
			deviceIdentifier,
			helpDepartment,
		))
	}
	return sb.String()
}

// GenerateTXTFilename creates a standardized filename based on date range and timestamp
// Format: HELP_YYYYMMDD_HHmm.txt, OT_YYYYMMDD_HHmm.txt, AT_YYYYMMDD_HHmm.txt
func GenerateTXTFilename(prefix string, start, end time.Time) string {
	bkk := GetBangkokLocation()
	if start.IsZero() {
		start = time.Now().In(bkk)
	}
	if end.IsZero() {
		end = start
	}

	normalizedPrefix := strings.ToUpper(strings.TrimSpace(prefix))
	switch normalizedPrefix {
	case "ATTENDANCE":
		normalizedPrefix = "AT"
	case "OVERTIME":
		normalizedPrefix = "OT"
	}

	startDateStr := start.In(bkk).Format("20060102")
	endDateStr := end.In(bkk).Format("20060102")
	timeStr := end.In(bkk).Format("1504")

	if startDateStr == endDateStr {
		return fmt.Sprintf("%s_%s_%s.txt", normalizedPrefix, startDateStr, timeStr)
	}
	return fmt.Sprintf("%s_%s_%s_%s.txt", normalizedPrefix, startDateStr, endDateStr, timeStr)
}

