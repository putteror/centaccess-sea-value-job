package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/putteror/sea-value/internal/core/model"
)

// FetchHelpRecords fetches help/overtime records from CentAccess API (/api/overtime/records) with automatic pagination and 401 retry
func (c *CentAccessAPIClient) FetchHelpRecords(ctx context.Context, start time.Time, end time.Time) ([]model.HelpRecord, error) {
	baseURL := c.cfg.AppOvertimeURL
	if baseURL == "" {
		baseURL = c.cfg.AppAPIURL
	}

	var allRecords []model.HelpRecord
	page := 1
	limit := 500
	const maxPages = 2000

	for page <= maxPages {
		params := url.Values{}
		params.Set("startCheckInTime", start.Format(time.RFC3339))
		params.Set("endCheckInTime", end.Format(time.RFC3339))
		params.Set("page", strconv.Itoa(page))
		params.Set("limit", strconv.Itoa(limit))

		reqURL := fmt.Sprintf("%s/api/overtime/records?%s", baseURL, params.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}

		authHeader, err := c.GetAuthToken(ctx)
		if err == nil && authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("http request error: %w", err)
		}

		// Handle 401 Unauthorized with automatic Token Refresh & Retry
		if resp.StatusCode == http.StatusUnauthorized {
			resp.Body.Close()
			newAuth, refErr := c.ForceRefreshToken(ctx)
			if refErr != nil {
				return nil, fmt.Errorf("help api returned 401 unauthorized: %w", refErr)
			}
			req.Header.Set("Authorization", newAuth)
			resp, err = c.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("retry request error: %w", err)
			}
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("help api returned status %d: %s", resp.StatusCode, string(body))
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read help response: %w", err)
		}

		var raw struct {
			Data []model.HelpRecord `json:"data"`
			Page struct {
				Page      int `json:"page"`
				Size      int `json:"size"`
				Total     int `json:"total"`
				TotalPage int `json:"totalPage"`
			} `json:"page"`
		}

		if err := json.Unmarshal(bodyBytes, &raw); err != nil {
			return nil, fmt.Errorf("failed to decode help response: %w", err)
		}

		allRecords = append(allRecords, raw.Data...)

		if len(raw.Data) == 0 || (raw.Page.TotalPage > 0 && page >= raw.Page.TotalPage) || len(raw.Data) < limit {
			break
		}
		page++
	}

	return allRecords, nil
}
