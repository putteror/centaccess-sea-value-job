package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/port"
)

type PreviewResult struct {
	Success            bool        `json:"success"`
	Endpoint           string      `json:"endpoint"`
	HTTPMethod         string      `json:"httpMethod"`
	StatusCode         int         `json:"statusCode"`
	StatusText         string      `json:"statusText"`
	LatencyMs          int64       `json:"latencyMs"`
	RecordCount        int         `json:"recordCount"`
	AttendanceRecords  int         `json:"attendanceRecords,omitempty"`
	OvertimeRecords    int         `json:"overtimeRecords,omitempty"`
	ExcludeDeviceNames []string    `json:"excludeDeviceNames,omitempty"`
	ExcludeDeviceIDs   []string    `json:"excludeDeviceIDs,omitempty"`
	TargetDeviceNames  []string    `json:"targetDeviceNames,omitempty"`
	TargetDeviceIDs    []string    `json:"targetDeviceIDs,omitempty"`
	TimeRange          string      `json:"timeRange"`
	RawResponse        interface{} `json:"rawResponse"`
	Error              string      `json:"error,omitempty"`
}

type ConnectionTestResult struct {
	Success     bool        `json:"success"`
	StatusCode  int         `json:"statusCode"`
	StatusText  string      `json:"statusText"`
	Endpoint    string      `json:"endpoint"`
	HTTPMethod  string      `json:"httpMethod"`
	LatencyMs   int64       `json:"latencyMs"`
	Username    string      `json:"username,omitempty"`
	Role        string      `json:"role,omitempty"`
	TenantName  string      `json:"tenantName,omitempty"`
	TenantID    string      `json:"tenantId,omitempty"`
	Message     string      `json:"message,omitempty"`
	Target      string      `json:"target,omitempty"`
	RawResponse interface{} `json:"rawResponse,omitempty"`
	Error       string      `json:"error,omitempty"`
}

type DiagnosticService interface {
	PreviewAttendance(ctx context.Context, start, end time.Time) (*PreviewResult, error)
	PreviewOvertime(ctx context.Context, start, end time.Time) (*PreviewResult, error)
	PreviewHelp(ctx context.Context, start, end time.Time) (*PreviewResult, error)
	TestConnection(ctx context.Context) (*ConnectionTestResult, error)
}

type diagnosticServiceImpl struct {
	cfg           *config.Config
	apiClient     port.CentAccessAPIPort
	deviceService DeviceService
	httpClient    *http.Client
}

func NewDiagnosticService(cfg *config.Config, apiClient port.CentAccessAPIPort, deviceService DeviceService) DiagnosticService {
	return &diagnosticServiceImpl{
		cfg:           cfg,
		apiClient:     apiClient,
		deviceService: deviceService,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *diagnosticServiceImpl) executeRequest(ctx context.Context, reqURL string) (int, string, int64, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return 0, "", 0, nil, err
	}

	authHeader, authErr := s.apiClient.GetAuthToken(ctx)
	if authErr == nil && authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	startExec := time.Now()
	resp, err := s.httpClient.Do(req)
	latency := time.Since(startExec).Milliseconds()

	if err == nil && resp.StatusCode == http.StatusUnauthorized {
		resp.Body.Close()
		newAuth, refErr := s.apiClient.ForceRefreshToken(ctx)
		if refErr == nil && newAuth != "" {
			req.Header.Set("Authorization", newAuth)
			startExec = time.Now()
			resp, err = s.httpClient.Do(req)
			latency = time.Since(startExec).Milliseconds()
		}
	}

	if err != nil {
		return 0, "", latency, nil, err
	}
	defer resp.Body.Close()

	bodyBytes, readErr := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Status, latency, bodyBytes, readErr
}

func (s *diagnosticServiceImpl) PreviewAttendance(ctx context.Context, start, end time.Time) (*PreviewResult, error) {
	params := url.Values{}
	params.Set("startDateTime", start.Format(time.RFC3339))
	params.Set("endDateTime", end.Format(time.RFC3339))
	params.Set("page", "1")
	params.Set("limit", "100")
	params.Set("result", "SUCCESS")

	reqURL := fmt.Sprintf("%s/api/access-records/?%s", s.cfg.AppAPIURL, params.Encode())
	statusCode, statusText, latency, bodyBytes, err := s.executeRequest(ctx, reqURL)

	timeRangeStr := fmt.Sprintf("%s to %s", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))

	if err != nil {
		return &PreviewResult{
			Success:    false,
			Endpoint:   reqURL,
			HTTPMethod: "GET",
			StatusCode: statusCode,
			LatencyMs:  latency,
			TimeRange:  timeRangeStr,
			Error:      fmt.Sprintf("Failed to connect to CentAccess App API: %v", err),
		}, nil
	}

	var rawJSON interface{}
	if jsonErr := json.Unmarshal(bodyBytes, &rawJSON); jsonErr != nil {
		rawJSON = string(bodyBytes)
	}

	var count int = 0
	var resDataArr struct {
		Data []struct {
			DeviceName string `json:"accessControlDeviceName"`
			DeviceID   string `json:"accessControlDeviceId"`
		} `json:"data"`
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	var nonOTCount int = 0
	if err := json.Unmarshal(bodyBytes, &resDataArr); err == nil {
		count = len(resDataArr.Data)
		if resDataArr.Page.Total > 0 {
			count = resDataArr.Page.Total
		}

		for _, item := range resDataArr.Data {
			if !s.deviceService.IsOTDevice(item.DeviceName, item.DeviceID) {
				nonOTCount++
			}
		}
	}

	return &PreviewResult{
		Success:            statusCode >= 200 && statusCode < 300,
		Endpoint:           reqURL,
		HTTPMethod:         "GET",
		StatusCode:         statusCode,
		StatusText:         statusText,
		LatencyMs:          latency,
		RecordCount:        count,
		AttendanceRecords:  nonOTCount,
		ExcludeDeviceNames: s.cfg.Jobs.Attendance.ExcludeDeviceNames,
		ExcludeDeviceIDs:   s.cfg.Jobs.Attendance.ExcludeDeviceIDs,
		TimeRange:          timeRangeStr,
		RawResponse:        rawJSON,
	}, nil
}

func (s *diagnosticServiceImpl) PreviewOvertime(ctx context.Context, start, end time.Time) (*PreviewResult, error) {
	params := url.Values{}
	params.Set("startDateTime", start.Format(time.RFC3339))
	params.Set("endDateTime", end.Format(time.RFC3339))
	params.Set("page", "1")
	params.Set("limit", "100")
	params.Set("result", "SUCCESS")

	reqURL := fmt.Sprintf("%s/api/access-records/?%s", s.cfg.AppAPIURL, params.Encode())
	statusCode, statusText, latency, bodyBytes, err := s.executeRequest(ctx, reqURL)

	timeRangeStr := fmt.Sprintf("%s to %s", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))

	if err != nil {
		return &PreviewResult{
			Success:    false,
			Endpoint:   reqURL,
			HTTPMethod: "GET",
			StatusCode: statusCode,
			LatencyMs:  latency,
			TimeRange:  timeRangeStr,
			Error:      fmt.Sprintf("Failed to connect to CentAccess App API: %v", err),
		}, nil
	}

	var rawJSON interface{}
	if jsonErr := json.Unmarshal(bodyBytes, &rawJSON); jsonErr != nil {
		rawJSON = string(bodyBytes)
	}

	var count int = 0
	var resDataArr struct {
		Data []struct {
			DeviceName string `json:"accessControlDeviceName"`
			DeviceID   string `json:"accessControlDeviceId"`
		} `json:"data"`
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	var otCount int = 0
	if err := json.Unmarshal(bodyBytes, &resDataArr); err == nil {
		count = len(resDataArr.Data)
		if resDataArr.Page.Total > 0 {
			count = resDataArr.Page.Total
		}

		for _, item := range resDataArr.Data {
			if s.deviceService.IsOTDevice(item.DeviceName, item.DeviceID) {
				otCount++
			}
		}
	}

	return &PreviewResult{
		Success:           statusCode >= 200 && statusCode < 300,
		Endpoint:          reqURL,
		HTTPMethod:        "GET",
		StatusCode:        statusCode,
		StatusText:        statusText,
		LatencyMs:         latency,
		RecordCount:       count,
		OvertimeRecords:   otCount,
		TargetDeviceNames: s.cfg.Jobs.Overtime.TargetDeviceNames,
		TargetDeviceIDs:   s.cfg.Jobs.Overtime.TargetDeviceIDs,
		TimeRange:         timeRangeStr,
		RawResponse:       rawJSON,
	}, nil
}

func (s *diagnosticServiceImpl) PreviewHelp(ctx context.Context, start, end time.Time) (*PreviewResult, error) {
	baseURL := s.cfg.AppOvertimeURL
	if baseURL == "" {
		baseURL = s.cfg.AppAPIURL
	}

	params := url.Values{}
	params.Set("startDate", start.Format("2006-01-02"))
	params.Set("endDate", end.Format("2006-01-02"))
	params.Set("page", "1")
	params.Set("limit", "100")

	reqURL := fmt.Sprintf("%s/api/help-records/?%s", baseURL, params.Encode())
	statusCode, statusText, latency, bodyBytes, err := s.executeRequest(ctx, reqURL)

	timeRangeStr := fmt.Sprintf("%s to %s", start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"))

	if err != nil {
		return &PreviewResult{
			Success:    false,
			Endpoint:   reqURL,
			HTTPMethod: "GET",
			StatusCode: statusCode,
			LatencyMs:  latency,
			TimeRange:  timeRangeStr,
			Error:      fmt.Sprintf("Failed to connect to Help API: %v", err),
		}, nil
	}

	var rawJSON interface{}
	if jsonErr := json.Unmarshal(bodyBytes, &rawJSON); jsonErr != nil {
		rawJSON = string(bodyBytes)
	}

	var count int = 0
	var resDataArr struct {
		Data []interface{} `json:"data"`
		Page struct {
			Total int `json:"total"`
		} `json:"page"`
	}
	if err := json.Unmarshal(bodyBytes, &resDataArr); err == nil {
		count = len(resDataArr.Data)
		if resDataArr.Page.Total > 0 {
			count = resDataArr.Page.Total
		}
	}

	return &PreviewResult{
		Success:     statusCode >= 200 && statusCode < 300,
		Endpoint:    reqURL,
		HTTPMethod:  "GET",
		StatusCode:  statusCode,
		StatusText:  statusText,
		LatencyMs:   latency,
		RecordCount: count,
		TimeRange:   timeRangeStr,
		RawResponse: rawJSON,
	}, nil
}

func (s *diagnosticServiceImpl) TestConnection(ctx context.Context) (*ConnectionTestResult, error) {
	if s.cfg.AppUsername != "" && s.cfg.AppPassword != "" {
		startTime := time.Now()
		loginRes, err := s.apiClient.Login(ctx, s.cfg.AppUsername, s.cfg.AppPassword)
		latency := time.Since(startTime).Milliseconds()

		if err != nil {
			return &ConnectionTestResult{
				Success:    false,
				StatusCode: 401,
				StatusText: "401 Unauthorized",
				Endpoint:   fmt.Sprintf("%s/api/login", s.cfg.AppAPIURL),
				HTTPMethod: "POST",
				LatencyMs:  latency,
				Username:   s.cfg.AppUsername,
				Error:      err.Error(),
			}, nil
		}

		return &ConnectionTestResult{
			Success:     true,
			StatusCode:  200,
			StatusText:  "200 OK",
			Endpoint:    fmt.Sprintf("%s/api/login", s.cfg.AppAPIURL),
			HTTPMethod:  "POST",
			LatencyMs:   latency,
			Username:    loginRes.Username,
			Role:        loginRes.Role,
			TenantName:  loginRes.TenantName,
			TenantID:    loginRes.TenantID,
			RawResponse: loginRes,
		}, nil
	}

	return &ConnectionTestResult{
		Success:    true,
		StatusCode: 200,
		StatusText: "200 OK",
		Message:    "Static APP_API_KEY is configured",
		Target:     s.cfg.AppAPIURL,
	}, nil
}
