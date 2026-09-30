package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type SyncOrchestratorService interface {
	TriggerManualSync(ctx context.Context, serviceType string, start, end time.Time) (string, error)
	TriggerContinueSync(ctx context.Context, serviceType string) ([]string, error)
	TriggerSyncAll(ctx context.Context, start, end time.Time) ([]string, error)
}

type syncOrchestratorServiceImpl struct {
	attendanceService AttendanceSyncService
	helpService       HelpSyncService
	peopleService     PeopleImportService
}

func NewSyncOrchestratorService(
	attendanceService AttendanceSyncService,
	helpService HelpSyncService,
	peopleService PeopleImportService,
) SyncOrchestratorService {
	return &syncOrchestratorServiceImpl{
		attendanceService: attendanceService,
		helpService:       helpService,
		peopleService:     peopleService,
	}
}

func (s *syncOrchestratorServiceImpl) TriggerManualSync(ctx context.Context, serviceType string, start, end time.Time) (string, error) {
	st := strings.ToLower(strings.TrimSpace(serviceType))
	switch st {
	case "attendance", "overtime", "access_records":
		if err := s.attendanceService.SyncAttendance(ctx, start, end, false); err != nil {
			return "", fmt.Errorf("attendance sync failed: %w", err)
		}
		return "Attendance & Overtime access records sync completed successfully.", nil

	case "help":
		if err := s.helpService.SyncHelp(ctx, start, end, false); err != nil {
			return "", fmt.Errorf("help records sync failed: %w", err)
		}
		return "Help records sync completed successfully.", nil

	case "people", "peopleimport":
		if err := s.peopleService.ProcessPendingExcelFiles(ctx); err != nil {
			return "", fmt.Errorf("people import failed: %w", err)
		}
		return "People Excel scan and import completed successfully.", nil

	default:
		return "", fmt.Errorf("unknown service type: %s", serviceType)
	}
}

func (s *syncOrchestratorServiceImpl) TriggerContinueSync(ctx context.Context, serviceType string) ([]string, error) {
	target := strings.ToLower(strings.TrimSpace(serviceType))
	if target == "" {
		target = "all"
	}

	var results []string
	var errs []string

	if target == "all" || target == "attendance" || target == "access_records" || target == "overtime" {
		if err := s.attendanceService.SyncAttendance(ctx, time.Time{}, time.Time{}, true); err != nil {
			errs = append(errs, fmt.Sprintf("Attendance: %v", err))
		} else {
			results = append(results, "Attendance & Overtime sync completed & watermarks updated.")
		}
	}

	if target == "all" || target == "help" {
		if err := s.helpService.SyncHelp(ctx, time.Time{}, time.Time{}, true); err != nil {
			errs = append(errs, fmt.Sprintf("Help: %v", err))
		} else {
			results = append(results, "Help records sync completed & watermarks updated.")
		}
	}

	if target == "all" || target == "people" || target == "peopleimport" {
		if err := s.peopleService.ProcessPendingExcelFiles(ctx); err != nil {
			errs = append(errs, fmt.Sprintf("People: %v", err))
		} else {
			results = append(results, "People Excel import completed.")
		}
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return results, nil
}

func (s *syncOrchestratorServiceImpl) TriggerSyncAll(ctx context.Context, start, end time.Time) ([]string, error) {
	var results []string
	var errs []string

	// 1. Attendance & Overtime
	if err := s.attendanceService.SyncAttendance(ctx, start, end, false); err != nil {
		errs = append(errs, fmt.Sprintf("Attendance: %v", err))
	} else {
		results = append(results, "Attendance & Overtime sync completed.")
	}

	// 2. Help
	if err := s.helpService.SyncHelp(ctx, start, end, false); err != nil {
		errs = append(errs, fmt.Sprintf("Help: %v", err))
	} else {
		results = append(results, "Help records sync completed.")
	}

	// 3. People
	if err := s.peopleService.ProcessPendingExcelFiles(ctx); err != nil {
		errs = append(errs, fmt.Sprintf("People: %v", err))
	} else {
		results = append(results, "People import completed.")
	}

	if len(errs) > 0 {
		return results, fmt.Errorf("%s", strings.Join(errs, "; "))
	}

	return results, nil
}
