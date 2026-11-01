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

// FetchHelpRecords fetches help/overtime records from CentAccess API (/api/overtime/records or fallback /api/help-records) with automatic pagination and 401 retry
func (c *CentAccessAPIClient) FetchHelpRecords(ctx context.Context, start time.Time, end time.Time) ([]model.HelpRecord, error) {
	baseURL := c.cfg.AppOvertimeURL
	if baseURL == "" {
		baseURL = c.cfg.AppAPIURL
	}

	var allRecords []model.HelpRecord
	page := 1
	limit := 500
	const maxPages = 2000
	endpointPath := "/api/overtime/records"

	for page <= maxPages {
		params := url.Values{}
		params.Set("startCheckInTime", start.Format(time.RFC3339))
		params.Set("endCheckInTime", end.Format(time.RFC3339))
		params.Set("startDateTime", start.Format(time.RFC3339))
		params.Set("endDateTime", end.Format(time.RFC3339))
		params.Set("startDate", start.Format("2006-01-02"))
		params.Set("endDate", end.Format("2006-01-02"))
		params.Set("page", strconv.Itoa(page))
		params.Set("limit", strconv.Itoa(limit))

		reqURL := fmt.Sprintf("%s%s?%s", baseURL, endpointPath, params.Encode())
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

		// If 404 on page 1, attempt fallback endpoint /api/help-records/
		if resp.StatusCode == http.StatusNotFound && page == 1 && endpointPath == "/api/overtime/records" {
			resp.Body.Close()
			endpointPath = "/api/help-records/"
			reqURL = fmt.Sprintf("%s%s?%s", baseURL, endpointPath, params.Encode())
			req, _ = http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
			if authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}
			resp, err = c.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("fallback help api request error: %w", err)
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

		// Try unmarshaling with support for multiple CentAccess response shapes:
		// Shape 1: { "data": [...], "page": { ... } }
		var pageRecords []model.HelpRecord
		var totalPage int

		var shape1 struct {
			Data []model.HelpRecord `json:"data"`
			Page struct {
				Page      int `json:"page"`
				Size      int `json:"size"`
				Total     int `json:"total"`
				TotalPage int `json:"totalPage"`
			} `json:"page"`
			Pagination struct {
				TotalPage int `json:"totalPage"`
			} `json:"pagination"`
			TotalPage int `json:"totalPage"`
		}

		if err := json.Unmarshal(bodyBytes, &shape1); err == nil && shape1.Data != nil {
			pageRecords = shape1.Data
			totalPage = shape1.Page.TotalPage
			if totalPage == 0 {
				totalPage = shape1.Pagination.TotalPage
			}
			if totalPage == 0 {
				totalPage = shape1.TotalPage
			}
		} else {
			// Shape 2: { "data": { "items": [...] } }
			var shape2 struct {
				Data struct {
					Items      []model.HelpRecord `json:"items"`
					Pagination struct {
						TotalPage int `json:"totalPage"`
					} `json:"pagination"`
					TotalPage int `json:"totalPage"`
				} `json:"data"`
			}
			if err2 := json.Unmarshal(bodyBytes, &shape2); err2 == nil && shape2.Data.Items != nil {
				pageRecords = shape2.Data.Items
				totalPage = shape2.Data.Pagination.TotalPage
				if totalPage == 0 {
					totalPage = shape2.Data.TotalPage
				}
			} else {
				// Shape 3: { "data": { "records": [...] } }
				var shape3 struct {
					Data struct {
						Records []model.HelpRecord `json:"records"`
					} `json:"data"`
				}
				if err3 := json.Unmarshal(bodyBytes, &shape3); err3 == nil && shape3.Data.Records != nil {
					pageRecords = shape3.Data.Records
				} else {
					// Shape 4: Direct array [ ... ]
					var shape4 []model.HelpRecord
					if err4 := json.Unmarshal(bodyBytes, &shape4); err4 == nil {
						pageRecords = shape4
					} else {
						return nil, fmt.Errorf("failed to decode help response: %w (body: %s)", err, string(bodyBytes))
					}
				}
			}
		}

		allRecords = append(allRecords, pageRecords...)

		if len(pageRecords) == 0 || (totalPage > 0 && page >= totalPage) || len(pageRecords) < limit {
			break
		}
		page++
	}

	return allRecords, nil
}
