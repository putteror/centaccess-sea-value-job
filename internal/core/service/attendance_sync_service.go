package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
	"github.com/putteror/sea-value/internal/core/port"
)

type ExportResult struct {
	Filename    string
	Content     []byte
	RecordCount int
}

type AttendanceSyncService interface {
	SyncAttendance(ctx context.Context, start time.Time, end time.Time, updateWatermark bool) error
	ExportAttendanceTXT(ctx context.Context, start, end time.Time, deviceIDs []string) (*ExportResult, error)
	ExportOvertimeTXT(ctx context.Context, start, end time.Time, deviceIDs []string) (*ExportResult, error)
}

type attendanceSyncServiceImpl struct {
	cfg       *config.Config
	apiClient port.CentAccessAPIPort
	storage   port.StoragePort
}

func NewAttendanceSyncService(cfg *config.Config, apiClient port.CentAccessAPIPort, storage port.StoragePort) AttendanceSyncService {
	return &attendanceSyncServiceImpl{
		cfg:       cfg,
		apiClient: apiClient,
		storage:   storage,
	}
}

func isOTDevice(deviceName, deviceID string, targetNames, targetIDs []string) bool {
	trimmedID := strings.TrimSpace(deviceID)
	for _, id := range targetIDs {
		tID := strings.TrimSpace(id)
		if tID != "" && strings.EqualFold(trimmedID, tID) {
			return true
		}
	}

	trimmedName := strings.TrimSpace(deviceName)
	lowerName := strings.ToLower(trimmedName)
	for _, name := range targetNames {
		tName := strings.TrimSpace(name)
		if tName != "" && (strings.EqualFold(trimmedName, tName) || strings.Contains(lowerName, strings.ToLower(tName))) {
			return true
		}
	}
	return false
}

func (s *attendanceSyncServiceImpl) SyncAttendance(ctx context.Context, start time.Time, end time.Time, updateWatermark bool) error {
	now := time.Now()
	if end.IsZero() {
		end = now
	}

	if start.IsZero() {
		// Use earliest attendance last sync time across all enabled devices, or fallback to configured initial lookback
		if earliest, ok := s.cfg.GetEarliestDeviceSyncTime("attendance"); ok && !earliest.IsZero() {
			start = earliest
		} else {
			start = s.cfg.GetInitialSyncTime("attendance")
		}
	}

	log.Printf("[ACCESS RECORDS SYNC] Fetching access records from %s to %s (updateWatermark: %v)...\n", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), updateWatermark)

	allRecords, err := s.apiClient.FetchAccessRecords(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to fetch access records: %w", err)
	}

	if len(allRecords) == 0 {
		log.Println("[ACCESS RECORDS SYNC] No access records found from API for the given time range.")
		if updateWatermark {
			// Update devices attendance last sync time to end if cron run and no new records
			_ = s.cfg.UpdateDevicesAttendanceSyncTime(nil, end)
		}
		return nil
	}

	// Determine OT device filters
	otNames := s.cfg.Jobs.Overtime.TargetDeviceNames
	if len(otNames) == 0 {
		otNames = s.cfg.Jobs.Attendance.ExcludeDeviceNames
	}
	otIDs := s.cfg.Jobs.Overtime.TargetDeviceIDs
	if len(otIDs) == 0 {
		otIDs = s.cfg.Jobs.Attendance.ExcludeDeviceIDs
	}

	// Split into Attendance (General) and Overtime (OT Gate) & track per-device max timestamps
	var attendanceRecords []model.AccessRecord
	var overtimeRecords []model.AccessRecord
	deviceMaxTimes := make(map[string]time.Time)

	// Build map of existing attendance last sync times per device
	deviceLastSyncMap := make(map[string]time.Time)
	for _, d := range s.cfg.GetDevices() {
		timeStr := strings.TrimSpace(d.AttendanceLastSyncTime)
		if timeStr == "" {
			timeStr = strings.TrimSpace(d.LastSyncTime)
		}
		if timeStr != "" {
			for _, fmtStr := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(fmtStr, timeStr); err == nil && !t.IsZero() {
					if d.ID != "" {
						deviceLastSyncMap[strings.TrimSpace(d.ID)] = t
					}
					if d.Name != "" {
						deviceLastSyncMap[strings.TrimSpace(d.Name)] = t
					}
					break
				}
			}
		}
	}

	for _, r := range allRecords {
		devID := strings.TrimSpace(r.AccessControlDeviceID)
		devName := strings.TrimSpace(r.AccessControlDeviceName)

		if devID != "" && r.RecordTime.After(deviceMaxTimes[devID]) {
			deviceMaxTimes[devID] = r.RecordTime
		}
		if devName != "" && r.RecordTime.After(deviceMaxTimes[devName]) {
			deviceMaxTimes[devName] = r.RecordTime
		}

		// If this device was already synced up to a later or equal timestamp, skip this record
		if updateWatermark {
			if lastTime, exists := deviceLastSyncMap[devID]; exists && !lastTime.IsZero() {
				if !r.RecordTime.After(lastTime) {
					continue
				}
			} else if lastTime, exists := deviceLastSyncMap[devName]; exists && !lastTime.IsZero() {
				if !r.RecordTime.After(lastTime) {
					continue
				}
			}
		}

		if isOTDevice(r.AccessControlDeviceName, r.AccessControlDeviceID, otNames, otIDs) {
			overtimeRecords = append(overtimeRecords, r)
		} else {
			attendanceRecords = append(attendanceRecords, r)
		}
	}

	var writeErr error

	// 1. Write non-OT attendance records to data/attendance/
	if len(attendanceRecords) > 0 {
		attPath, err := s.storage.WriteAttendanceRecords(ctx, attendanceRecords, end)
		if err != nil {
			log.Printf("[ERROR] Failed to write attendance records: %v\n", err)
			writeErr = err
		} else {
			log.Printf("[ATTENDANCE SYNC] Written %d records to '%s'\n", len(attendanceRecords), attPath)
		}
	} else {
		log.Printf("[ATTENDANCE SYNC] 0 non-OT records found in %d total fetched records.\n", len(allRecords))
	}

	// 2. Write OT gate records to data/overtime/
	if len(overtimeRecords) > 0 {
		otPath, err := s.storage.WriteOvertimeRecords(ctx, overtimeRecords, end)
		if err != nil {
			log.Printf("[ERROR] Failed to write overtime gate records: %v\n", err)
			if writeErr == nil {
				writeErr = err
			}
		} else {
			log.Printf("[OVERTIME GATE SYNC] Written %d records to '%s'\n", len(overtimeRecords), otPath)
		}
	} else {
		log.Printf("[OVERTIME GATE SYNC] 0 OT gate records matched in %d total fetched records.\n", len(allRecords))
	}

	// Update devices attendance last sync time watermarks strictly if updateWatermark is enabled (Cron)
	if updateWatermark {
		_ = s.cfg.UpdateDevicesAttendanceSyncTime(deviceMaxTimes, end)
	}

	return writeErr
}

func (s *attendanceSyncServiceImpl) ExportAttendanceTXT(ctx context.Context, start, end time.Time, deviceIDs []string) (*ExportResult, error) {
	return s.exportAccessRecordsTXT(ctx, false, start, end, deviceIDs)
}

func (s *attendanceSyncServiceImpl) ExportOvertimeTXT(ctx context.Context, start, end time.Time, deviceIDs []string) (*ExportResult, error) {
	return s.exportAccessRecordsTXT(ctx, true, start, end, deviceIDs)
}

func (s *attendanceSyncServiceImpl) exportAccessRecordsTXT(ctx context.Context, isOvertime bool, start, end time.Time, deviceIDs []string) (*ExportResult, error) {
	bkk := model.GetBangkokLocation()
	if start.IsZero() {
		start = time.Now().In(bkk).Truncate(24 * time.Hour)
	}
	if end.IsZero() {
		end = time.Now().In(bkk)
	}

	allRecords, err := s.apiClient.FetchAccessRecords(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch access records: %w", err)
	}

	deviceFilterMap := make(map[string]bool)
	for _, id := range deviceIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			deviceFilterMap[trimmed] = true
		}
	}

	otNames := s.cfg.Jobs.Overtime.TargetDeviceNames
	if len(otNames) == 0 {
		otNames = s.cfg.Jobs.Attendance.ExcludeDeviceNames
	}
	otIDs := s.cfg.Jobs.Overtime.TargetDeviceIDs
	if len(otIDs) == 0 {
		otIDs = s.cfg.Jobs.Attendance.ExcludeDeviceIDs
	}

	var filteredRecords []model.AccessRecord
	for _, r := range allRecords {
		devID := strings.TrimSpace(r.AccessControlDeviceID)
		devName := strings.TrimSpace(r.AccessControlDeviceName)
		if len(deviceFilterMap) > 0 && !deviceFilterMap[devID] && !deviceFilterMap[devName] {
			continue
		}

		isOT := isOTDevice(r.AccessControlDeviceName, r.AccessControlDeviceID, otNames, otIDs)
		if isOvertime && !isOT {
			continue
		}
		if !isOvertime && isOT {
			continue
		}

		filteredRecords = append(filteredRecords, r)
	}

	prefix := "attendance"
	if isOvertime {
		prefix = "overtime"
	}

	content := model.FormatAccessRecordsToTXT(filteredRecords)
	filename := model.GenerateTXTFilename(prefix, start, end)

	return &ExportResult{
		Filename:    filename,
		Content:     []byte(content),
		RecordCount: len(filteredRecords),
	}, nil
}
