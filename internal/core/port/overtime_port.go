package port

import (
	"context"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

// OvertimeAPIPort handles fetching access records from CentAccess
type OvertimeAPIPort interface {
	FetchAccessRecords(ctx context.Context, start time.Time, end time.Time) ([]model.AccessRecord, error)
}

// OvertimeStoragePort handles writing formatted overtime records (from AccessRecord) to local text files
type OvertimeStoragePort interface {
	WriteOvertimeRecords(ctx context.Context, records []model.AccessRecord, targetDate time.Time) (string, error)
}
