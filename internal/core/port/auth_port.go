package port

import (
	"context"

	"github.com/putteror/sea-value/internal/core/model"
)

// AuthAPIPort handles login and token caching / refresh with CentAccess
type AuthAPIPort interface {
	Login(ctx context.Context, username, password string) (*model.LoginResponseData, error)
	GetAuthToken(ctx context.Context) (string, error)
	ForceRefreshToken(ctx context.Context) (string, error)
}
