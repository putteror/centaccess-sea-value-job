package model

import (
	"encoding/json"
	"strings"
	"time"
)

// HasCheckInAndOut returns true if the record has both valid check-in and check-out timestamps and is not just checked-in
func (h HelpRecord) HasCheckInAndOut() bool {
	if strings.EqualFold(h.Status, "CHECKED_IN") {
		return false
	}
	return h.CheckInTime != nil && !h.CheckInTime.IsZero() && h.CheckOutTime != nil && !h.CheckOutTime.IsZero()
}

// HelpRecord represents CentAccess Help / Overtime Check-in / Check-out entry matching App API
type HelpRecord struct {
	ID                      string     `json:"id"`
	OTSessionID             string     `json:"otSessionId"`
	TenantID                string     `json:"tenantId"`
	PersonID                string     `json:"personId"`
	PersonName              string     `json:"personName"`
	PersonCode              string     `json:"personCode"`
	PersonDepartment        string     `json:"personDepartment"`
	DepartmentCode          string     `json:"departmentCode"`
	DepartmentName          string     `json:"departmentName"`
	HelpDepartment          string     `json:"helpDepartment,omitempty"`
	HelpDepartmentCode      string     `json:"helpDepartmentCode,omitempty"`
	SessionCardNumber       string     `json:"sessionCardNumber"`
	CardNumber              string     `json:"cardNumber"`
	AccessMethod            string     `json:"accessMethod"`
	CheckInTime             *time.Time `json:"checkInTime"`
	CheckOutTime            *time.Time `json:"checkOutTime"`
	DurationMinutes         int        `json:"durationMinutes"`
	Status                  string     `json:"status"` // "CHECKED_IN" | "CHECKED_OUT" | "AUTO_CLOSED"
	AccessControlDeviceID   string     `json:"accessControlDeviceId"`
	AccessControlDeviceCode string     `json:"accessControlDeviceCode,omitempty"`
	AccessControlDeviceName string     `json:"accessControlDeviceName"`
	SessionStatus           string     `json:"sessionStatus"`
	CreatedAt               time.Time  `json:"createdAt"`
	UpdatedAt               time.Time  `json:"updatedAt"`
}

// UnmarshalJSON implements custom JSON unmarshaling to support flexible datetime parsing,
// AccessControlDeviceCode, code, and deviceCode aliases
func (h *HelpRecord) UnmarshalJSON(data []byte) error {
	type Alias HelpRecord
	aux := struct {
		*Alias
		Code               string      `json:"code"`
		DeviceCode         string      `json:"deviceCode"`
		DeviceID           string      `json:"deviceId"`
		DeviceName         string      `json:"deviceName"`
		EmployeeCode       string      `json:"employeeCode"`
		EmpCode            string      `json:"empCode"`
		CheckInRaw         interface{} `json:"checkInTime"`
		CheckinRaw         interface{} `json:"checkinTime"`
		CheckInUnderRaw    interface{} `json:"check_in_time"`
		StartTimeRaw       interface{} `json:"startTime"`
		CheckOutRaw        interface{} `json:"checkOutTime"`
		CheckoutRaw        interface{} `json:"checkoutTime"`
		CheckOutUnderRaw   interface{} `json:"check_out_time"`
		EndTimeRaw         interface{} `json:"endTime"`
		CreatedAtRaw       interface{} `json:"createdAt"`
		UpdatedAtRaw       interface{} `json:"updatedAt"`
	}{
		Alias: (*Alias)(h),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if h.AccessControlDeviceCode == "" {
		if aux.Code != "" {
			h.AccessControlDeviceCode = aux.Code
		} else if aux.DeviceCode != "" {
			h.AccessControlDeviceCode = aux.DeviceCode
		}
	}
	if h.AccessControlDeviceID == "" && aux.DeviceID != "" {
		h.AccessControlDeviceID = aux.DeviceID
	}
	if h.AccessControlDeviceName == "" && aux.DeviceName != "" {
		h.AccessControlDeviceName = aux.DeviceName
	}
	if h.PersonCode == "" {
		if aux.EmployeeCode != "" {
			h.PersonCode = aux.EmployeeCode
		} else if aux.EmpCode != "" {
			h.PersonCode = aux.EmpCode
		}
	}

	// Parse CheckInTime
	for _, raw := range []interface{}{aux.CheckInRaw, aux.CheckinRaw, aux.CheckInUnderRaw, aux.StartTimeRaw} {
		if t := parseFlexibleTime(raw); t != nil {
			h.CheckInTime = t
			break
		}
	}

	// Parse CheckOutTime
	for _, raw := range []interface{}{aux.CheckOutRaw, aux.CheckoutRaw, aux.CheckOutUnderRaw, aux.EndTimeRaw} {
		if t := parseFlexibleTime(raw); t != nil {
			h.CheckOutTime = t
			break
		}
	}

	// Parse CreatedAt / UpdatedAt if zero
	if h.CreatedAt.IsZero() {
		if t := parseFlexibleTime(aux.CreatedAtRaw); t != nil {
			h.CreatedAt = *t
		}
	}
	if h.UpdatedAt.IsZero() {
		if t := parseFlexibleTime(aux.UpdatedAtRaw); t != nil {
			h.UpdatedAt = *t
		}
	}

	return nil
}

// parseFlexibleTime attempts to parse various datetime representations into a *time.Time
func parseFlexibleTime(val interface{}) *time.Time {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case time.Time:
		return &v
	case *time.Time:
		return v
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil
		}
		formats := []string{
			time.RFC3339Nano,
			time.RFC3339,
			"2006-01-02T15:04:05.999999999",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05.999999999",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, f := range formats {
			if t, err := time.Parse(f, s); err == nil {
				return &t
			}
		}
	case float64:
		// Epoch timestamp (seconds or milliseconds)
		if v > 1e11 {
			t := time.UnixMilli(int64(v))
			return &t
		} else if v > 0 {
			t := time.Unix(int64(v), 0)
			return &t
		}
	case int64:
		if v > 1e11 {
			t := time.UnixMilli(v)
			return &t
		} else if v > 0 {
			t := time.Unix(v, 0)
			return &t
		}
	}
	return nil
}

// OvertimeRecord alias for backward compatibility
type OvertimeRecord = HelpRecord
