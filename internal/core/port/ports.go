package port

// CentAccessAPIPort combines all domain API ports (Attendance, Device, Help, People, Auth)
type CentAccessAPIPort interface {
	AttendanceAPIPort
	DeviceAPIPort
	HelpAPIPort
	PeopleAPIPort
	AuthAPIPort
}

// StoragePort combines file storage ports (Attendance, Overtime, Help)
type StoragePort interface {
	AttendanceStoragePort
	OvertimeStoragePort
	HelpStoragePort
}
