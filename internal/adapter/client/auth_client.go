package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/putteror/sea-value/internal/core/model"
)

// GetAuthToken returns a valid Authorization header ("Bearer <token>").
// Reads from config.json/memory first. If empty, automatically calls Login and persists it.
func (c *CentAccessAPIClient) GetAuthToken(ctx context.Context) (string, error) {
	c.tokenMu.RLock()
	if c.cfg.AppToken != "" {
		token := c.cfg.AppToken
		c.tokenMu.RUnlock()
		return "Bearer " + token, nil
	}
	if c.cfg.AppAPIKey != "" {
		token := c.cfg.AppAPIKey
		c.tokenMu.RUnlock()
		return "Bearer " + token, nil
	}
	c.tokenMu.RUnlock()

	// If username & password are provided, perform auto-login & save to config.json
	if c.cfg.AppUsername != "" && c.cfg.AppPassword != "" {
		loginRes, err := c.Login(ctx, c.cfg.AppUsername, c.cfg.AppPassword)
		if err != nil {
			return "", fmt.Errorf("auto-login failed: %w", err)
		}
		return "Bearer " + loginRes.Token, nil
	}

	return "", fmt.Errorf("no authentication credentials configured (APP_API_KEY or APP_USERNAME/APP_PASSWORD)")
}

// ForceRefreshToken forces a re-login to update the cached JWT token in config.json upon 401
func (c *CentAccessAPIClient) ForceRefreshToken(ctx context.Context) (string, error) {
	if c.cfg.AppUsername == "" || c.cfg.AppPassword == "" {
		return "", fmt.Errorf("cannot refresh token: APP_USERNAME or APP_PASSWORD not configured")
	}

	loginRes, err := c.Login(ctx, c.cfg.AppUsername, c.cfg.AppPassword)
	if err != nil {
		return "", fmt.Errorf("refresh token login failed: %w", err)
	}

	return "Bearer " + loginRes.Token, nil
}

// Login authenticates credentials with CentAccess API (POST /api/login)
// and persists the JWT token to config.json and memory
func (c *CentAccessAPIClient) Login(ctx context.Context, username, password string) (*model.LoginResponseData, error) {
	reqURL := fmt.Sprintf("%s/api/login", c.cfg.AppAPIURL)
	loginReq := model.LoginRequest{
		Username: username,
		Password: password,
	}

	payloadBytes, err := json.Marshal(loginReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal login payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("login request error: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read login response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, string(bodyBytes))
	}

	// 1. Try Direct JSON format: { "token": "...", "username": "...", ... }
	var directRes model.LoginResponseData
	if err := json.Unmarshal(bodyBytes, &directRes); err == nil && directRes.Token != "" {
		_ = c.cfg.SetAppToken(directRes.Token)
		return &directRes, nil
	}

	// 2. Try Wrapped JSON format: { "data": { "token": "...", ... } }
	var wrappedRes struct {
		Data model.LoginResponseData `json:"data"`
	}
	if err := json.Unmarshal(bodyBytes, &wrappedRes); err == nil && wrappedRes.Data.Token != "" {
		_ = c.cfg.SetAppToken(wrappedRes.Data.Token)
		return &wrappedRes.Data, nil
	}

	return nil, fmt.Errorf("login response did not contain a valid token: %s", string(bodyBytes))
}
