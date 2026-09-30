package storage

import (
	"sync"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/port"
)

var (
	bangkokLocation *time.Location
	locOnce         sync.Once
)

// GetBangkokLocation returns Asia/Bangkok (UTC+7) timezone location with fallback
func GetBangkokLocation() *time.Location {
	locOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Bangkok")
		if err != nil {
			loc = time.FixedZone("Asia/Bangkok", 7*3600)
		}
		bangkokLocation = loc
	})
	return bangkokLocation
}

func getBangkokLocation() *time.Location {
	return GetBangkokLocation()
}


type FileStorageAdapter struct {
	cfg *config.Config
}

func NewFileStorageAdapter(cfg *config.Config) port.StoragePort {
	return &FileStorageAdapter{
		cfg: cfg,
	}
}
