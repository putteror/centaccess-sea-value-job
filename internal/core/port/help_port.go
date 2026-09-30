package port

import (
	"context"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

// HelpAPIPort handles fetching help records from CentAccess App API (/api/overtime/records)
type HelpAPIPort interface {
	FetchHelpRecords(ctx context.Context, start time.Time, end time.Time) ([]model.HelpRecord, error)
}

// HelpStoragePort handles writing formatted help records to local text files
type HelpStoragePort interface {
	WriteHelpRecords(ctx context.Context, records []model.HelpRecord, targetDate time.Time) (string, error)
}
