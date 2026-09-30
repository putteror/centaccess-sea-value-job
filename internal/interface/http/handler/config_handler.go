package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/model"
	"github.com/putteror/sea-value/internal/core/service"
	appcron "github.com/putteror/sea-value/internal/cron"
	"github.com/putteror/sea-value/internal/interface/http/web"
	"github.com/robfig/cron/v3"
)

type ConfigHandler struct {
	cfg               *config.Config
	attendanceService service.AttendanceSyncService
	helpService       service.HelpSyncService
	deviceService     service.DeviceService
	diagnosticService service.DiagnosticService
	syncOrchestrator  service.SyncOrchestratorService
	scheduler         *appcron.Scheduler
}

func NewConfigHandler(
	cfg *config.Config,
	attendanceService service.AttendanceSyncService,
	helpService service.HelpSyncService,
	deviceService service.DeviceService,
	diagnosticService service.DiagnosticService,
	syncOrchestrator service.SyncOrchestratorService,
	scheduler *appcron.Scheduler,
) *ConfigHandler {
	return &ConfigHandler{
		cfg:               cfg,
		attendanceService: attendanceService,
		helpService:       helpService,
		deviceService:     deviceService,
		diagnosticService: diagnosticService,
		syncOrchestrator:  syncOrchestrator,
		scheduler:         scheduler,
	}
}

// 1. Web UI Page View
func (h *ConfigHandler) ServeConfigPage(w http.ResponseWriter, r *http.Request) {
	tmpl, err := web.GetTemplate()
	if err != nil {
		http.Error(w, "Failed to render template: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := struct {
		AppAPIURL      string
		AppOvertimeURL string
		HTTPPort       string
		ExportBaseDir  string
		LogLevel       string
		Schedules      []config.ScheduleConfig
		Devices        []model.DeviceItem
		ActiveTime     string
	}{
		AppAPIURL:      h.cfg.AppAPIURL,
		AppOvertimeURL: h.cfg.AppOvertimeURL,
		HTTPPort:       h.cfg.HTTPPort,
		ExportBaseDir:  h.cfg.ExportBaseDir,
		LogLevel:       h.cfg.LogLevel,
		Schedules:      h.cfg.GetSchedules(),
		Devices:        h.cfg.GetDevices(),
		ActiveTime:     time.Now().Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.Execute(w, data)
}

// 2. Config JSON Read / Update
func (h *ConfigHandler) GetConfigJSON(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"httpPort":       h.cfg.HTTPPort,
		"exportBaseDir":  h.cfg.ExportBaseDir,
		"logLevel":       h.cfg.LogLevel,
		"appApiUrl":      h.cfg.AppAPIURL,
		"appOvertimeUrl": h.cfg.AppOvertimeURL,
		"appToken":       h.cfg.AppToken,
		"schedules":      h.cfg.GetSchedules(),
		"jobs":           h.cfg.Jobs,
		"devices":        h.cfg.GetDevices(),
	})
}

type UpdateConfigRequest struct {
	HTTPPort      string                  `json:"httpPort"`
	ExportBaseDir string                  `json:"exportBaseDir"`
	LogLevel      string                  `json:"logLevel"`
	Schedules     []config.ScheduleConfig `json:"schedules"`
	Jobs          *config.JobsConfig      `json:"jobs,omitempty"`
}

func (h *ConfigHandler) UpdateConfigJSON(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	cleanedSchedules := make([]config.ScheduleConfig, 0, len(req.Schedules))
	for i, sch := range req.Schedules {
		trimmedName := strings.TrimSpace(sch.Name)
		if trimmedName == "" {
			trimmedName = fmt.Sprintf("Schedule #%d", i+1)
		}
		trimmedType := strings.ToLower(strings.TrimSpace(sch.Type))
		if trimmedType != "attendance" && trimmedType != "overtime" && trimmedType != "help" && trimmedType != "people" && trimmedType != "peopleimport" {
			trimmedType = "attendance"
		}
		trimmedCron := strings.TrimSpace(sch.CronSchedule)
		if trimmedCron == "" {
			http.Error(w, fmt.Sprintf("Schedule '%s' has an empty cron expression", trimmedName), http.StatusBadRequest)
			return
		}

		if _, err := parser.Parse(trimmedCron); err != nil {
			http.Error(w, fmt.Sprintf("Invalid cron expression '%s' for '%s': %v", trimmedCron, trimmedName, err), http.StatusBadRequest)
			return
		}

		id := sch.ID
		if id == "" {
			id = fmt.Sprintf("sch_%d_%d", time.Now().Unix(), i+1)
		}

		cleanedSchedules = append(cleanedSchedules, config.ScheduleConfig{
			ID:           id,
			Name:         trimmedName,
			Type:         trimmedType,
			CronSchedule: trimmedCron,
			Enabled:      sch.Enabled,
			LastRunAt:    sch.LastRunAt,
		})
	}

	currentJSON := config.JSONConfigFile{
		HTTPPort:      h.cfg.HTTPPort,
		ExportBaseDir: h.cfg.ExportBaseDir,
		LogLevel:      h.cfg.LogLevel,
		AppToken:      h.cfg.AppToken,
		Schedules:     cleanedSchedules,
		Jobs:          h.cfg.Jobs,
	}

	if req.Jobs != nil {
		currentJSON.Jobs = *req.Jobs
	}
	if req.HTTPPort != "" {
		currentJSON.HTTPPort = req.HTTPPort
	}
	if req.ExportBaseDir != "" {
		currentJSON.ExportBaseDir = req.ExportBaseDir
	}

	if err := h.cfg.SaveConfig(currentJSON); err != nil {
		http.Error(w, "Failed to save config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if h.scheduler != nil {
		if err := h.scheduler.Reload(); err != nil {
			http.Error(w, "Config saved but failed to reload scheduler: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"message":   "Configuration updated and Scheduler reloaded seamlessly.",
		"schedules": h.cfg.GetSchedules(),
	})
}

// 3. Manual Sync Endpoints
func (h *ConfigHandler) TriggerAttendanceSync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	start, end := parseTimeRange(r)
	msg, err := h.syncOrchestrator.TriggerManualSync(ctx, "attendance", start, end)
	h.renderSyncResponse(w, msg, err)
}

func (h *ConfigHandler) TriggerOvertimeSync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	start, end := parseTimeRange(r)
	msg, err := h.syncOrchestrator.TriggerManualSync(ctx, "attendance", start, end)
	h.renderSyncResponse(w, msg, err)
}

func (h *ConfigHandler) TriggerHelpSync(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	start, end := parseTimeRange(r)
	msg, err := h.syncOrchestrator.TriggerManualSync(ctx, "help", start, end)
	h.renderSyncResponse(w, msg, err)
}

func (h *ConfigHandler) TriggerPeopleImport(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	msg, err := h.syncOrchestrator.TriggerManualSync(ctx, "people", time.Time{}, time.Time{})
	h.renderSyncResponse(w, msg, err)
}

func (h *ConfigHandler) TriggerSyncAll(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	start, end := parseTimeRange(r)
	results, err := h.syncOrchestrator.TriggerSyncAll(ctx, start, end)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"results": results,
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "All synchronization tasks triggered successfully.",
		"time":    time.Now().Format("2006-01-02 15:04:05"),
		"results": results,
	})
}

// 4. Continue Sync Mode (Watermark-based)
func (h *ConfigHandler) TriggerContinueSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	var req struct {
		Service string `json:"service"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	results, err := h.syncOrchestrator.TriggerContinueSync(ctx, req.Service)

	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"results": results,
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Continue Sync completed successfully.",
		"time":    time.Now().Format("2006-01-02 15:04:05"),
		"results": results,
	})
}

// 5. Independent Sync Mode (.txt File Export)
type ExportTXTRequest struct {
	Service   string   `json:"service"`
	Start     string   `json:"start"`
	End       string   `json:"end"`
	DeviceIDs []string `json:"deviceIds"`
}

func (h *ConfigHandler) ExportTXTFile(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()

	var req ExportTXTRequest
	if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	if req.Service == "" {
		req.Service = r.URL.Query().Get("service")
	}
	if req.Start == "" {
		req.Start = r.URL.Query().Get("startDateTime")
	}
	if req.End == "" {
		req.End = r.URL.Query().Get("endDateTime")
	}
	if len(req.DeviceIDs) == 0 && r.URL.Query().Get("deviceIds") != "" {
		req.DeviceIDs = strings.Split(r.URL.Query().Get("deviceIds"), ",")
	}

	start, end := parseExplicitTimeRange(req.Start, req.End)
	serviceType := strings.ToLower(strings.TrimSpace(req.Service))

	var result *service.ExportResult
	var err error

	switch serviceType {
	case "help":
		result, err = h.helpService.ExportHelpTXT(ctx, start, end, req.DeviceIDs)
	case "overtime":
		result, err = h.attendanceService.ExportOvertimeTXT(ctx, start, end, req.DeviceIDs)
	default: // attendance
		result, err = h.attendanceService.ExportAttendanceTXT(ctx, start, end, req.DeviceIDs)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Export failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", result.Filename))
	_, _ = w.Write(result.Content)
}

// 6. Test Fetch / Preview Endpoints
func (h *ConfigHandler) TestFetchAttendance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	start, end := parseTimeRange(r)
	result, err := h.diagnosticService.PreviewAttendance(ctx, start, end)
	h.renderDiagnosticResponse(w, result, err)
}

func (h *ConfigHandler) TestFetchOvertime(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	start, end := parseTimeRange(r)
	result, err := h.diagnosticService.PreviewOvertime(ctx, start, end)
	h.renderDiagnosticResponse(w, result, err)
}

func (h *ConfigHandler) TestFetchHelp(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	start, end := parseTimeRange(r)
	result, err := h.diagnosticService.PreviewHelp(ctx, start, end)
	h.renderDiagnosticResponse(w, result, err)
}

func (h *ConfigHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	result, err := h.diagnosticService.TestConnection(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	if !result.Success && result.StatusCode > 0 {
		w.WriteHeader(result.StatusCode)
	}
	_ = json.NewEncoder(w).Encode(result)
}

// 7. Device Registry Endpoints
func (h *ConfigHandler) GetDevices(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	devices, err := h.deviceService.GetDevices(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":              true,
		"devices":              devices,
		"targetDeviceNames":    h.cfg.Jobs.Overtime.TargetDeviceNames,
		"targetDeviceIds":      h.cfg.Jobs.Overtime.TargetDeviceIDs,
		"initialLookbackHours": h.cfg.Jobs.Attendance.InitialLookbackHours,
		"maxLookbackDays":      h.cfg.Jobs.Attendance.MaxLookbackDays,
	})
}

func (h *ConfigHandler) SyncDevicesFromCentAccess(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	devices, err := h.deviceService.SyncDevicesFromCentAccess(ctx)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":              true,
		"message":              fmt.Sprintf("Discovered and updated %d devices from CentAccess API.", len(devices)),
		"devices":              devices,
		"targetDeviceNames":    h.cfg.Jobs.Overtime.TargetDeviceNames,
		"targetDeviceIds":      h.cfg.Jobs.Overtime.TargetDeviceIDs,
		"initialLookbackHours": h.cfg.Jobs.Attendance.InitialLookbackHours,
		"maxLookbackDays":      h.cfg.Jobs.Attendance.MaxLookbackDays,
	})
}

type UpdateDevicesConfigRequest struct {
	Devices              []model.DeviceItem `json:"devices"`
	TargetDeviceNames    []string           `json:"targetDeviceNames"`
	TargetDeviceIDs      []string           `json:"targetDeviceIds"`
	InitialLookbackHours *int               `json:"initialLookbackHours,omitempty"`
	MaxLookbackDays      *int               `json:"maxLookbackDays,omitempty"`
}

func (h *ConfigHandler) UpdateDevicesConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req UpdateDevicesConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.deviceService.UpdateDevicesConfig(ctx, req.Devices, req.TargetDeviceNames, req.TargetDeviceIDs, req.InitialLookbackHours, req.MaxLookbackDays); err != nil {
		http.Error(w, "Failed to save device config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":           true,
		"message":           "Device configuration updated successfully.",
		"targetDeviceNames": req.TargetDeviceNames,
		"targetDeviceIds":   req.TargetDeviceIDs,
	})
}

// 8. Watermark Adjustment Endpoint
func (h *ConfigHandler) ResetDeviceWatermark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID               string  `json:"deviceId"`
		AttendanceLastSyncTime *string `json:"attendanceLastSyncTime,omitempty"`
		HelpLastSyncTime       *string `json:"helpLastSyncTime,omitempty"`
		Type                   string  `json:"type,omitempty"`      // legacy: "attendance" | "help" | "all"
		ResetTime              *string `json:"resetTime,omitempty"` // custom time or empty
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	var attPtr *string
	var helpPtr *string

	if req.AttendanceLastSyncTime != nil {
		attStr := normalizeTimeStr(*req.AttendanceLastSyncTime)
		attPtr = &attStr
	}
	if req.HelpLastSyncTime != nil {
		helpStr := normalizeTimeStr(*req.HelpLastSyncTime)
		helpPtr = &helpStr
	}

	// Support legacy format or single resetTime
	if attPtr == nil && helpPtr == nil {
		customTimeStr := ""
		if req.ResetTime != nil {
			customTimeStr = normalizeTimeStr(*req.ResetTime)
		}
		targetType := strings.ToLower(req.Type)
		if targetType == "attendance" {
			attPtr = &customTimeStr
		} else if targetType == "help" {
			helpPtr = &customTimeStr
		} else {
			attPtr = &customTimeStr
			helpPtr = &customTimeStr
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if err := h.deviceService.SetDeviceWatermarks(ctx, req.DeviceID, attPtr, helpPtr); err != nil {
		http.Error(w, "Failed to update device watermark: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Device sync watermark updated successfully.",
	})
}

// 9. Helpers
func (h *ConfigHandler) renderSyncResponse(w http.ResponseWriter, msg string, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
			"message": err.Error(),
		})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": msg,
		"time":    time.Now().Format("2006-01-02 15:04:05"),
	})
}

func (h *ConfigHandler) renderDiagnosticResponse(w http.ResponseWriter, result *service.PreviewResult, err error) {
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}
	if !result.Success && result.StatusCode > 0 {
		w.WriteHeader(result.StatusCode)
	}
	_ = json.NewEncoder(w).Encode(result)
}

func parseTimeRange(r *http.Request) (time.Time, time.Time) {
	return parseExplicitTimeRange(r.URL.Query().Get("startDateTime"), r.URL.Query().Get("endDateTime"))
}

func parseExplicitTimeRange(startStr, endStr string) (time.Time, time.Time) {
	now := time.Now()
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endTime := now

	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		time.RFC3339,
	}

	if startStr != "" {
		for _, f := range formats {
			if t, err := time.ParseInLocation(f, strings.TrimSpace(startStr), now.Location()); err == nil {
				startTime = t
				break
			}
		}
	}
	if endStr != "" {
		for _, f := range formats {
			if t, err := time.ParseInLocation(f, strings.TrimSpace(endStr), now.Location()); err == nil {
				endTime = t
				break
			}
		}
	}
	return startTime, endTime
}

func normalizeTimeStr(val string) string {
	val = strings.TrimSpace(val)
	if val == "" {
		return ""
	}
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, fmtStr := range formats {
		if t, err := time.ParseInLocation(fmtStr, val, time.Local); err == nil {
			return t.Format(time.RFC3339)
		}
	}
	return val
}
