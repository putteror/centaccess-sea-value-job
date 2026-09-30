package model

import "time"

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
	CardNumber             string    `json:"cardNumber"`
	AccessControlDeviceID  string    `json:"accessControlDeviceId"`
	AccessControlDeviceName string   `json:"accessControlDeviceName"`
	AccessMethod           string    `json:"accessMethod"`
	Result                 string    `json:"result"` // "GRANTED" | "DENIED"
	RecordTime             time.Time `json:"recordTime"`
	CaptureImagePath       string    `json:"captureImagePath,omitempty"`
	PersonImagePath        string    `json:"personImagePath,omitempty"`
}

// DeviceItem represents an access control device discovered from CentAccess and persisted in system
type DeviceItem struct {
	ID                     string `json:"id"`
	Name                   string `json:"name"`
	IPAddress              string `json:"ipAddress,omitempty"`
	Status                 string `json:"status,omitempty"`
	IsOT                   bool   `json:"isOt"`
	Enabled                bool   `json:"enabled"`
	AttendanceLastSyncTime string `json:"attendanceLastSyncTime,omitempty"`
	HelpLastSyncTime       string `json:"helpLastSyncTime,omitempty"`
	LastSyncTime           string `json:"lastSyncTime,omitempty"` // backward compatibility fallback
}

