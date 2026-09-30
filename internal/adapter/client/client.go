package client

import (
	"net/http"
	"sync"
	"time"

	"github.com/putteror/sea-value/internal/config"
	"github.com/putteror/sea-value/internal/core/port"
)

type CentAccessAPIClient struct {
	cfg        *config.Config
	httpClient *http.Client
	tokenMu    sync.RWMutex
}

func NewCentAccessAPIClient(cfg *config.Config) port.CentAccessAPIPort {
	return &CentAccessAPIClient{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}
