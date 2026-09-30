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

// FetchAccessRecords fetches door/face access records from CentAccess API with automatic pagination, config-based page size, and 401 retry
func (c *CentAccessAPIClient) FetchAccessRecords(ctx context.Context, start time.Time, end time.Time) ([]model.AccessRecord, error) {
	var allRecords []model.AccessRecord
	page := 1
	
	// Determine page size from configuration if available
	limit := 500
	if c.cfg != nil && c.cfg.Jobs.Attendance.FetchLimit > 0 {
		limit = c.cfg.Jobs.Attendance.FetchLimit
		if limit > 2000 {
			limit = 2000 // Safety upper cap for single HTTP payload
		}
	}

	const maxPages = 2000 // Safety guard against runaway loops

	for page <= maxPages {
		params := url.Values{}
		params.Set("startDateTime", start.Format(time.RFC3339))
		params.Set("endDateTime", end.Format(time.RFC3339))
		params.Set("page", strconv.Itoa(page))
		params.Set("limit", strconv.Itoa(limit))

		reqURL := fmt.Sprintf("%s/api/access-records/?%s", c.cfg.AppAPIURL, params.Encode())
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
				return nil, fmt.Errorf("api returned 401 unauthorized and refresh token failed: %w", refErr)
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
			return nil, fmt.Errorf("api returned status %d: %s", resp.StatusCode, string(body))
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var raw struct {
			Data []model.AccessRecord `json:"data"`
			Page struct {
				Page      int `json:"page"`
				Size      int `json:"size"`
				Total     int `json:"total"`
				TotalPage int `json:"totalPage"`
			} `json:"page"`
			Pagination struct {
				Page      int `json:"page"`
				Limit     int `json:"limit"`
				Total     int `json:"total"`
				TotalPage int `json:"totalPage"`
			} `json:"pagination"`
		}

		if err := json.Unmarshal(bodyBytes, &raw); err != nil {
			var alt struct {
				Data struct {
					Items []model.AccessRecord `json:"items"`
				} `json:"data"`
			}
			if altErr := json.Unmarshal(bodyBytes, &alt); altErr == nil && len(alt.Data.Items) > 0 {
				allRecords = append(allRecords, alt.Data.Items...)
				break
			}
			return nil, fmt.Errorf("failed to parse access records: %w", err)
		}

		allRecords = append(allRecords, raw.Data...)

		totalPage := raw.Page.TotalPage
		if totalPage == 0 {
			totalPage = raw.Pagination.TotalPage
		}

		// Stop when no data or reached final page
		if len(raw.Data) == 0 || (totalPage > 0 && page >= totalPage) || len(raw.Data) < limit {
			break
		}
		page++
	}

	return allRecords, nil
}
