package model

import (
	"encoding/json"
	"time"
)

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

// UnmarshalJSON implements custom JSON unmarshaling to support AccessControlDeviceCode, code, and deviceCode
func (h *HelpRecord) UnmarshalJSON(data []byte) error {
	type Alias HelpRecord
	aux := struct {
		*Alias
		Code       string `json:"code"`
		DeviceCode string `json:"deviceCode"`
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
	return nil
}

// OvertimeRecord alias for backward compatibility
type OvertimeRecord = HelpRecord
