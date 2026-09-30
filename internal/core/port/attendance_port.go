package port

import (
	"context"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

// AttendanceAPIPort handles fetching attendance & door access records from CentAccess
type AttendanceAPIPort interface {
	FetchAccessRecords(ctx context.Context, start time.Time, end time.Time) ([]model.AccessRecord, error)
}

// AttendanceStoragePort handles writing formatted attendance records to local text files
type AttendanceStoragePort interface {
	WriteAttendanceRecords(ctx context.Context, records []model.AccessRecord, targetDate time.Time) (string, error)
}
