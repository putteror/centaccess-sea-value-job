package cron

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/service"
	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	mu                sync.Mutex
	cron              *cron.Cron
	cfg               *config.Config
	attendanceService service.AttendanceSyncService
	helpService       service.HelpSyncService
	peopleService     service.PeopleImportService
}

func NewScheduler(
	cfg *config.Config,
	attendanceService service.AttendanceSyncService,
	helpService service.HelpSyncService,
	peopleService service.PeopleImportService,
) *Scheduler {
	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	return &Scheduler{
		cron:              cron.New(cron.WithParser(parser)),
		cfg:               cfg,
		attendanceService: attendanceService,
		helpService:       helpService,
		peopleService:     peopleService,
	}
}

func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	schedules := s.cfg.GetSchedules()
	registeredCount := 0

	for _, sch := range schedules {
		if !sch.Enabled {
			continue
		}

		item := sch // Pin loop variable
		var taskFunc func()

		switch strings.ToLower(item.Type) {
		case "attendance", "access_records", "overtime":
			taskFunc = func() {
				ctx := context.Background()
				now := time.Now()

				// Retrieve earliest device attendance sync timestamp, or fallback to initial lookback
				start, hasLastSync := s.cfg.GetEarliestDeviceSyncTime("attendance")
				if !hasLastSync || start.IsZero() || start.After(now) {
					start = s.cfg.GetInitialSyncTime("attendance")
				}

				log.Printf("[CRON TRIGGER] '%s' (Type: Attendance & OT Gate, Cron: %s, Range: %s -> %s)\n",
					item.Name, item.CronSchedule, start.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"))

				if err := s.attendanceService.SyncAttendance(ctx, start, now, true); err != nil {
					log.Printf("[CRON ERROR: %s] %v\n", item.Name, err)
				} else {
					log.Printf("[CRON SUCCESS: %s] Completed at %s\n", item.Name, time.Now().Format("15:04:05"))
					s.cfg.RecordScheduleLastRun(item.ID, now)
				}
			}
		case "help":
			taskFunc = func() {
				ctx := context.Background()
				now := time.Now()

				// Retrieve earliest device help sync timestamp, or fallback to initial lookback
				start, hasLastSync := s.cfg.GetEarliestDeviceSyncTime("help")
				if !hasLastSync || start.IsZero() || start.After(now) {
					start = s.cfg.GetInitialSyncTime("help")
				}

				log.Printf("[CRON TRIGGER] '%s' (Type: Help Records, Cron: %s, Range: %s -> %s)\n",
					item.Name, item.CronSchedule, start.Format("2006-01-02 15:04:05"), now.Format("2006-01-02 15:04:05"))

				if err := s.helpService.SyncHelp(ctx, start, now, true); err != nil {
					log.Printf("[CRON ERROR: %s] %v\n", item.Name, err)
				} else {
					log.Printf("[CRON SUCCESS: %s] Completed at %s\n", item.Name, time.Now().Format("15:04:05"))
					s.cfg.RecordScheduleLastRun(item.ID, now)
				}
			}
		case "people", "peopleimport":
			taskFunc = func() {
				ctx := context.Background()
				now := time.Now()
				log.Printf("[CRON TRIGGER] '%s' (Type: People Import, Cron: %s)\n", item.Name, item.CronSchedule)
				if err := s.peopleService.ProcessPendingExcelFiles(ctx); err != nil {
					log.Printf("[CRON ERROR: %s] %v\n", item.Name, err)
				} else {
					log.Printf("[CRON SUCCESS: %s] Completed at %s\n", item.Name, time.Now().Format("15:04:05"))
					s.cfg.RecordScheduleLastRun(item.ID, now)
				}
			}
		default:
			log.Printf("[CRON WARNING] Unknown job type '%s' for schedule '%s'\n", item.Type, item.Name)
			continue
		}

		entryID, err := s.cron.AddFunc(item.CronSchedule, taskFunc)
		if err != nil {
			log.Printf("[CRON ERROR] Failed to register '%s' (Cron: '%s'): %v\n", item.Name, item.CronSchedule, err)
			return fmt.Errorf("invalid cron expression '%s' for schedule '%s': %w", item.CronSchedule, item.Name, err)
		}

		registeredCount++
		log.Printf("[CRON REGISTERED #%d] ID: %s | %s [%s] -> %s (EntryID: %d)\n", registeredCount, item.ID, item.Name, item.Type, item.CronSchedule, entryID)
	}

	s.cron.Start()
	log.Printf("[CRON] Scheduler started with %d active jobs.\n", registeredCount)
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		s.cron.Stop()
		log.Println("[CRON] Schedulers stopped.")
	}
}

// Reload restarts the scheduler with updated in-memory configuration
func (s *Scheduler) Reload() error {
	s.Stop()

	parser := cron.NewParser(
		cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	s.mu.Lock()
	s.cron = cron.New(cron.WithParser(parser))
	s.mu.Unlock()

	return s.Start()
}
