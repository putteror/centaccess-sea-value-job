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

type HelpSyncService interface {
	SyncHelp(ctx context.Context, start time.Time, end time.Time, updateWatermark bool) error
	ExportHelpTXT(ctx context.Context, start, end time.Time, deviceIDs []string) (*ExportResult, error)
}

type helpSyncServiceImpl struct {
	cfg       *config.Config
	apiClient port.CentAccessAPIPort
	storage   port.StoragePort
}

func NewHelpSyncService(cfg *config.Config, apiClient port.CentAccessAPIPort, storage port.StoragePort) HelpSyncService {
	return &helpSyncServiceImpl{
		cfg:       cfg,
		apiClient: apiClient,
		storage:   storage,
	}
}

func (s *helpSyncServiceImpl) SyncHelp(ctx context.Context, start time.Time, end time.Time, updateWatermark bool) error {
	now := time.Now()
	if end.IsZero() {
		end = now
	}

	if start.IsZero() {
		// Use earliest help last sync time across all enabled devices, or fallback to configured initial lookback
		if earliest, ok := s.cfg.GetEarliestDeviceSyncTime("help"); ok && !earliest.IsZero() {
			start = earliest
		} else {
			start = s.cfg.GetInitialSyncTime("help")
		}
	}

	log.Printf("[TASK 3: HELP RECORDS] Fetching help records from %s to %s (updateWatermark: %v)...\n", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), updateWatermark)

	records, err := s.apiClient.FetchHelpRecords(ctx, start, end)
	if err != nil {
		return fmt.Errorf("failed to fetch help records: %w", err)
	}

	if len(records) == 0 {
		log.Println("[TASK 3: HELP RECORDS] No help records found.")
		if updateWatermark {
			// Update devices help last sync time to end if cron run and no new records
			_ = s.cfg.UpdateDevicesHelpSyncTime(nil, end)
		}
		return nil
	}

	// Track max timestamp per device and filter records already synced
	deviceMaxTimes := make(map[string]time.Time)

	// Build map of existing help last sync times and device code lookup per device
	deviceLastSyncMap := make(map[string]time.Time)
	deviceCodeLookup := make(map[string]string)
	for _, d := range s.cfg.GetDevices() {
		code := d.GetCode()
		if code != "" {
			if d.ID != "" {
				deviceCodeLookup[strings.TrimSpace(d.ID)] = code
			}
			if d.Name != "" {
				deviceCodeLookup[strings.TrimSpace(d.Name)] = code
			}
		}

		timeStr := strings.TrimSpace(d.HelpLastSyncTime)
		if timeStr != "" {
			for _, fmtStr := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
				if t, err := time.Parse(fmtStr, timeStr); err == nil && !t.IsZero() {
					if d.ID != "" {
						deviceLastSyncMap[strings.TrimSpace(d.ID)] = t
					}
					if code != "" {
						deviceLastSyncMap[strings.TrimSpace(code)] = t
					}
					if d.Name != "" {
						deviceLastSyncMap[strings.TrimSpace(d.Name)] = t
					}
					break
				}
			}
		}
	}

	var validRecords []model.HelpRecord
	for _, r := range records {
		if !r.HasCheckInAndOut() {
			continue
		}

		devID := strings.TrimSpace(r.AccessControlDeviceID)
		devName := strings.TrimSpace(r.AccessControlDeviceName)

		if r.AccessControlDeviceCode == "" {
			if c, ok := deviceCodeLookup[devID]; ok {
				r.AccessControlDeviceCode = c
			} else if c, ok := deviceCodeLookup[devName]; ok {
				r.AccessControlDeviceCode = c
			}
		}
		devCode := strings.TrimSpace(r.AccessControlDeviceCode)

		var recTime time.Time
		if r.CheckInTime != nil && !r.CheckInTime.IsZero() {
			recTime = *r.CheckInTime
		} else if !r.CreatedAt.IsZero() {
			recTime = r.CreatedAt
		}

		// If this device was already synced up to a later or equal timestamp, skip this record
		if updateWatermark && !recTime.IsZero() {
			var lastTime time.Time
			var hasLastTime bool

			if t, exists := deviceLastSyncMap[devID]; exists && !t.IsZero() {
				lastTime, hasLastTime = t, true
			} else if devCode != "" && deviceLastSyncMap[devCode] != (time.Time{}) {
				lastTime, hasLastTime = deviceLastSyncMap[devCode], true
			} else if t, exists := deviceLastSyncMap[devName]; exists && !t.IsZero() {
				lastTime, hasLastTime = t, true
			} else {
				// Substring / case-insensitive device matching fallback
				for _, d := range s.cfg.GetDevices() {
					if devName != "" && d.Name != "" && (strings.EqualFold(d.Name, devName) || strings.Contains(strings.ToLower(d.Name), strings.ToLower(devName)) || strings.Contains(strings.ToLower(devName), strings.ToLower(d.Name))) {
						if t, exists := deviceLastSyncMap[strings.TrimSpace(d.ID)]; exists && !t.IsZero() {
							lastTime, hasLastTime = t, true
							break
						}
					}
				}
			}

			if hasLastTime && isRecordAlreadySynced(recTime, lastTime) {
				continue
			}
		}

		// Track max record timestamp per device only for new processed records
		if devID != "" && !recTime.IsZero() && recTime.After(deviceMaxTimes[devID]) {
			deviceMaxTimes[devID] = recTime
		}
		if devCode != "" && !recTime.IsZero() && recTime.After(deviceMaxTimes[devCode]) {
			deviceMaxTimes[devCode] = recTime
		}
		if devName != "" && !recTime.IsZero() && recTime.After(deviceMaxTimes[devName]) {
			deviceMaxTimes[devName] = recTime
		}
		validRecords = append(validRecords, r)
	}

	if len(validRecords) > 0 {
		filePath, err := s.storage.WriteHelpRecords(ctx, validRecords, end)
		if err != nil {
			return fmt.Errorf("failed to write help records to file: %w", err)
		}
		log.Printf("[TASK 3 SUCCESS] Written %d help records to '%s'\n", len(validRecords), filePath)
	} else {
		log.Printf("[TASK 3 SUCCESS] 0 new help records to write after per-device watermark filtering.\n")
	}

	// Update devices help last sync time watermarks strictly if updateWatermark is enabled (Cron)
	if updateWatermark {
		_ = s.cfg.UpdateDevicesHelpSyncTime(deviceMaxTimes, end)
	}

	return nil
}

func (s *helpSyncServiceImpl) ExportHelpTXT(ctx context.Context, start, end time.Time, deviceIDs []string) (*ExportResult, error) {
	bkk := model.GetBangkokLocation()
	if start.IsZero() {
		start = time.Now().In(bkk).Truncate(24 * time.Hour)
	}
	if end.IsZero() {
		end = time.Now().In(bkk)
	}

	records, err := s.apiClient.FetchHelpRecords(ctx, start, end)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch help records: %w", err)
	}

	deviceFilterMap := make(map[string]bool)
	for _, id := range deviceIDs {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			deviceFilterMap[trimmed] = true
		}
	}

	deviceCodeLookup := make(map[string]string)
	for _, d := range s.cfg.GetDevices() {
		code := d.GetCode()
		if code != "" {
			if d.ID != "" {
				deviceCodeLookup[strings.TrimSpace(d.ID)] = code
			}
			if d.Name != "" {
				deviceCodeLookup[strings.TrimSpace(d.Name)] = code
			}
		}
	}

	var validRecords []model.HelpRecord
	for _, r := range records {
		if !r.HasCheckInAndOut() {
			continue
		}

		devID := strings.TrimSpace(r.AccessControlDeviceID)
		devName := strings.TrimSpace(r.AccessControlDeviceName)

		if r.AccessControlDeviceCode == "" {
			if c, ok := deviceCodeLookup[devID]; ok {
				r.AccessControlDeviceCode = c
			} else if c, ok := deviceCodeLookup[devName]; ok {
				r.AccessControlDeviceCode = c
			}
		}
		devCode := strings.TrimSpace(r.AccessControlDeviceCode)

		if len(deviceFilterMap) > 0 && !deviceFilterMap[devID] && !deviceFilterMap[devCode] && !deviceFilterMap[devName] {
			continue
		}
		validRecords = append(validRecords, r)
	}

	content := model.FormatHelpRecordsToTXT(validRecords, start)
	filename := model.GenerateTXTFilename("HELP", start, end)

	return &ExportResult{
		Filename:    filename,
		Content:     []byte(content),
		RecordCount: len(validRecords),
	}, nil
}
