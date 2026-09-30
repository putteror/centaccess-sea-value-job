package port

import (
	"context"

	"github.com/putteror/sea-value/internal/core/model"
)

// DeviceAPIPort handles fetching and discovering access control devices from CentAccess
type DeviceAPIPort interface {
	FetchDevices(ctx context.Context) ([]model.DeviceItem, error)
}
