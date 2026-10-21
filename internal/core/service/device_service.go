package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
	"github.com/putteror/sea-value/internal/core/port"
)

// DeviceService manages device registry, discovery, OT classification, and sync watermarks
type DeviceService interface {
	GetDevices(ctx context.Context) ([]model.DeviceItem, error)
	SyncDevicesFromCentAccess(ctx context.Context) ([]model.DeviceItem, error)
	UpdateDevicesConfig(ctx context.Context, devices []model.DeviceItem, targetNames, targetIDs []string, initialLookbackHours, maxLookbackDays *int) error
	SetDeviceWatermarks(ctx context.Context, deviceID string, attTime, helpTime *string) error
	IsOTDevice(deviceName, deviceID string, deviceCode ...string) bool
}

type deviceServiceImpl struct {
	cfg       *config.Config
	apiClient port.DeviceAPIPort
}

func NewDeviceService(cfg *config.Config, apiClient port.DeviceAPIPort) DeviceService {
	return &deviceServiceImpl{
		cfg:       cfg,
		apiClient: apiClient,
	}
}

// IsOTDevice checks whether a device matches the configured OT keywords, codes, or target IDs
func (s *deviceServiceImpl) IsOTDevice(deviceName, deviceID string, deviceCode ...string) bool {
	targetNames := s.cfg.Jobs.Overtime.TargetDeviceNames
	targetIDs := s.cfg.Jobs.Overtime.TargetDeviceIDs
	targetCodes := s.cfg.Jobs.Overtime.TargetDeviceCodes

	var code string
	if len(deviceCode) > 0 {
		code = strings.TrimSpace(deviceCode[0])
	}

	// 1. Check TargetDeviceCodes
	if code != "" {
		for _, c := range targetCodes {
			tC := strings.TrimSpace(c)
			if tC != "" && strings.EqualFold(code, tC) {
				return true
			}
		}
	}

	// 2. Check TargetDeviceIDs
	trimmedID := strings.TrimSpace(deviceID)
	for _, id := range targetIDs {
		tID := strings.TrimSpace(id)
		if tID != "" && (strings.EqualFold(trimmedID, tID) || (code != "" && strings.EqualFold(code, tID))) {
			return true
		}
	}

	// 3. Check TargetDeviceNames (also check keyword in code)
	trimmedName := strings.TrimSpace(deviceName)
	lowerName := strings.ToLower(trimmedName)
	lowerCode := strings.ToLower(code)
	for _, name := range targetNames {
		tName := strings.TrimSpace(name)
		if tName != "" {
			lowerTarget := strings.ToLower(tName)
			if strings.EqualFold(trimmedName, tName) || strings.Contains(lowerName, lowerTarget) {
				return true
			}
			if code != "" && (strings.EqualFold(code, tName) || strings.Contains(lowerCode, lowerTarget)) {
				return true
			}
		}
	}
	return false
}

// GetDevices returns all devices from local registry, or auto-discovers if registry is empty
func (s *deviceServiceImpl) GetDevices(ctx context.Context) ([]model.DeviceItem, error) {
	devices := s.cfg.GetDevices()

	if len(devices) == 0 && s.apiClient != nil {
		discovered, err := s.apiClient.FetchDevices(ctx)
		if err == nil && len(discovered) > 0 {
			for i := range discovered {
				discovered[i].Enabled = true
				if s.IsOTDevice(discovered[i].Name, discovered[i].ID, discovered[i].GetCode()) {
					discovered[i].IsOT = true
				}
			}
			_ = s.cfg.SaveDevices(discovered)
			devices = discovered
		}
	} else {
		for i := range devices {
			if s.IsOTDevice(devices[i].Name, devices[i].ID, devices[i].GetCode()) {
				devices[i].IsOT = true
			}
		}
	}

	return devices, nil
}

// SyncDevicesFromCentAccess fetches fresh devices from API and merges them with existing configs
func (s *deviceServiceImpl) SyncDevicesFromCentAccess(ctx context.Context) ([]model.DeviceItem, error) {
	discovered, err := s.apiClient.FetchDevices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to sync devices from CentAccess: %w", err)
	}

	existing := s.cfg.GetDevices()
	existingMap := make(map[string]model.DeviceItem)
	for _, d := range existing {
		if d.ID != "" {
			existingMap[d.ID] = d
		}
		if d.GetCode() != "" {
			existingMap[d.GetCode()] = d
		}
		if d.Name != "" {
			existingMap[d.Name] = d
		}
	}

	for i := range discovered {
		d := &discovered[i]
		var old model.DeviceItem
		var found bool
		if o, ok := existingMap[d.ID]; ok && d.ID != "" {
			old, found = o, true
		} else if o, ok := existingMap[d.GetCode()]; ok && d.GetCode() != "" {
			old, found = o, true
		} else if o, ok := existingMap[d.Name]; ok && d.Name != "" {
			old, found = o, true
		}

		if found {
			d.IsOT = old.IsOT
			d.Enabled = old.Enabled
			d.AttendanceLastSyncTime = old.AttendanceLastSyncTime
			d.HelpLastSyncTime = old.HelpLastSyncTime
			d.LastSyncTime = old.LastSyncTime
			if d.GetCode() == "" && old.GetCode() != "" {
				d.Code = old.GetCode()
				d.AccessControlDeviceCode = old.GetCode()
			}
		} else {
			d.Enabled = true
			if s.IsOTDevice(d.Name, d.ID, d.GetCode()) {
				d.IsOT = true
			}
		}
	}

	if err := s.cfg.SaveDevices(discovered); err != nil {
		return nil, fmt.Errorf("failed to save devices to disk: %w", err)
	}

	return discovered, nil
}

// UpdateDevicesConfig updates devices and OT matching rules in devices.json and config.json
func (s *deviceServiceImpl) UpdateDevicesConfig(ctx context.Context, devices []model.DeviceItem, targetNames, targetIDs []string, initialLookbackHours, maxLookbackDays *int) error {
	for i := range devices {
		if s.IsOTDevice(devices[i].Name, devices[i].ID, devices[i].GetCode()) {
			devices[i].IsOT = true
		}
	}

	if err := s.cfg.SaveDevices(devices); err != nil {
		return fmt.Errorf("failed to save devices: %w", err)
	}

	fileBytes, err := os.ReadFile(s.cfg.ConfigFilePath)
	if err == nil {
		var jsonCfg config.JSONConfigFile
		if err := json.Unmarshal(fileBytes, &jsonCfg); err == nil {
			changed := false
			if len(targetNames) > 0 {
				jsonCfg.Jobs.Overtime.TargetDeviceNames = targetNames
				jsonCfg.Jobs.Attendance.ExcludeDeviceNames = targetNames
				changed = true
			}
			if len(targetIDs) > 0 {
				jsonCfg.Jobs.Overtime.TargetDeviceIDs = targetIDs
				jsonCfg.Jobs.Attendance.ExcludeDeviceIDs = targetIDs
				changed = true
			}
			if initialLookbackHours != nil {
				jsonCfg.Jobs.Attendance.InitialLookbackHours = *initialLookbackHours
				jsonCfg.Jobs.Overtime.InitialLookbackHours = *initialLookbackHours
				jsonCfg.Jobs.Help.InitialLookbackHours = *initialLookbackHours
				changed = true
			}
			if maxLookbackDays != nil {
				jsonCfg.Jobs.Attendance.MaxLookbackDays = *maxLookbackDays
				jsonCfg.Jobs.Overtime.MaxLookbackDays = *maxLookbackDays
				jsonCfg.Jobs.Help.MaxLookbackDays = *maxLookbackDays
				changed = true
			}
			if changed {
				_ = s.cfg.SaveConfig(jsonCfg)
			}
		}
	}

	return nil
}

// SetDeviceWatermarks updates or resets watermark timestamps for a device or all devices
func (s *deviceServiceImpl) SetDeviceWatermarks(ctx context.Context, deviceID string, attTime, helpTime *string) error {
	return s.cfg.SetDeviceSyncTime(deviceID, attTime, helpTime)
}
