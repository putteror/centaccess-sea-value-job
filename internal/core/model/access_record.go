package model

import (
	"encoding/json"
	"time"
)

// Pagination represents standard API pagination metadata
type Pagination struct {
	Page      int `json:"page"`
	Limit     int `json:"limit"`
	Total     int `json:"total"`
	TotalPage int `json:"totalPage"`
}

// PaginatedList represents standard API data list with pagination
type PaginatedList[T any] struct {
	Items      []T        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// AccessRecord represents Core Access Scan record matching CentAccess API specification
type AccessRecord struct {
	ID                     string    `json:"id"`
	PersonID               string    `json:"personId"`
	PersonName             string    `json:"personName"`
	PersonCode             string    `json:"personCode"`
	Company                string    `json:"company"`
	Department             string    `json:"department"`
	JobPosition            string    `json:"jobPosition"`
	CardNumber              string    `json:"cardNumber"`
	AccessControlDeviceID   string    `json:"accessControlDeviceId"`
	AccessControlDeviceCode string    `json:"accessControlDeviceCode,omitempty"`
	AccessControlDeviceName string    `json:"accessControlDeviceName"`
	AccessMethod            string    `json:"accessMethod"`
	Result                  string    `json:"result"` // "SUCCESS" | "FAILED"
	RecordTime              time.Time `json:"recordTime"`
	CaptureImagePath        string    `json:"captureImagePath,omitempty"`
	PersonImagePath         string    `json:"personImagePath,omitempty"`
}

// DeviceItem represents an access control device discovered from CentAccess and persisted in system
type DeviceItem struct {
	ID                      string `json:"id"`
	AccessControlDeviceCode string `json:"accessControlDeviceCode,omitempty"`
	Code                    string `json:"code,omitempty"`
	Name                    string `json:"name"`
	IPAddress               string `json:"ipAddress,omitempty"`
	Status                  string `json:"status,omitempty"`
	IsOT                    bool   `json:"isOt"`
	Enabled                 bool   `json:"enabled"`
	AttendanceLastSyncTime  string `json:"attendanceLastSyncTime,omitempty"`
	HelpLastSyncTime        string `json:"helpLastSyncTime,omitempty"`
	LastSyncTime            string `json:"lastSyncTime,omitempty"` // backward compatibility fallback
}

// GetCode returns the device code supporting both AccessControlDeviceCode and Code
func (d DeviceItem) GetCode() string {
	if d.AccessControlDeviceCode != "" {
		return d.AccessControlDeviceCode
	}
	return d.Code
}

// UnmarshalJSON implements custom JSON unmarshaling to support AccessControlDeviceCode, code, and deviceCode
func (r *AccessRecord) UnmarshalJSON(data []byte) error {
	type Alias AccessRecord
	aux := struct {
		*Alias
		Code       string `json:"code"`
		DeviceCode string `json:"deviceCode"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if r.AccessControlDeviceCode == "" {
		if aux.Code != "" {
			r.AccessControlDeviceCode = aux.Code
		} else if aux.DeviceCode != "" {
			r.AccessControlDeviceCode = aux.DeviceCode
		}
	}
	return nil
}

// UnmarshalJSON implements custom JSON unmarshaling to ensure AccessControlDeviceCode and Code are both populated
func (d *DeviceItem) UnmarshalJSON(data []byte) error {
	type Alias DeviceItem
	aux := struct {
		*Alias
		DeviceCode string `json:"deviceCode"`
	}{
		Alias: (*Alias)(d),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if d.AccessControlDeviceCode == "" {
		if d.Code != "" {
			d.AccessControlDeviceCode = d.Code
		} else if aux.DeviceCode != "" {
			d.AccessControlDeviceCode = aux.DeviceCode
		}
	}
	if d.Code == "" {
		d.Code = d.AccessControlDeviceCode
	}
	return nil
}


