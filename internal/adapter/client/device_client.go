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

// FetchDevices retrieves all access control devices from CentAccess API with auto-pagination and discovery fallback
func (c *CentAccessAPIClient) FetchDevices(ctx context.Context) ([]model.DeviceItem, error) {
	deviceMap := make(map[string]model.DeviceItem)

	// 1. Direct Devices API endpoints: /api/access-control-devices or /api/devices with Auto-Pagination
	deviceEndpoints := []string{
		fmt.Sprintf("%s/api/access-control-devices", c.cfg.AppAPIURL),
		fmt.Sprintf("%s/api/devices", c.cfg.AppAPIURL),
	}

	for _, endpointURL := range deviceEndpoints {
		page := 1
		pageSize := 500
		const maxDevicePages = 100

		for page <= maxDevicePages {
			params := url.Values{}
			params.Set("page", strconv.Itoa(page))
			params.Set("limit", strconv.Itoa(pageSize))

			reqURL := fmt.Sprintf("%s?%s", endpointURL, params.Encode())
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
			if err != nil {
				break
			}

			authHeader, _ := c.GetAuthToken(ctx)
			if authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}

			resp, err := c.httpClient.Do(req)
			if err != nil {
				break
			}

			// Handle 401 Unauthorized with token refresh & retry
			if resp.StatusCode == http.StatusUnauthorized {
				resp.Body.Close()
				newAuth, refErr := c.ForceRefreshToken(ctx)
				if refErr != nil {
					break
				}
				req.Header.Set("Authorization", newAuth)
				resp, err = c.httpClient.Do(req)
				if err != nil {
					break
				}
			}

			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				break
			}

			bodyBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				break
			}

			// Parse response formats
			var res struct {
				Data []struct {
					ID         string `json:"id"`
					DeviceID   string `json:"deviceId"`
					Name       string `json:"name"`
					DeviceName string `json:"deviceName"`
					IPAddress  string `json:"ipAddress"`
					IP         string `json:"ip"`
					Status     string `json:"status"`
				} `json:"data"`
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

			if parseErr := json.Unmarshal(bodyBytes, &res); parseErr == nil && len(res.Data) > 0 {
				for _, d := range res.Data {
					id := d.ID
					if id == "" {
						id = d.DeviceID
					}
					name := d.Name
					if name == "" {
						name = d.DeviceName
					}
					ip := d.IPAddress
					if ip == "" {
						ip = d.IP
					}
					if id != "" || name != "" {
						key := id
						if key == "" {
							key = name
						}
						deviceMap[key] = model.DeviceItem{
							ID:        id,
							Name:      name,
							IPAddress: ip,
							Status:    d.Status,
						}
					}
				}

				totalPage := res.Page.TotalPage
				if totalPage == 0 {
					totalPage = res.Pagination.TotalPage
				}

				// If non-paginated flat list or reached the end of pages, stop pagination
				if totalPage == 0 || page >= totalPage || len(res.Data) < pageSize {
					break
				}
				page++
			} else {
				// Alternative nested data.items format
				var alt struct {
					Data struct {
						Items []struct {
							ID         string `json:"id"`
							DeviceID   string `json:"deviceId"`
							Name       string `json:"name"`
							DeviceName string `json:"deviceName"`
							IPAddress  string `json:"ipAddress"`
							IP         string `json:"ip"`
							Status     string `json:"status"`
						} `json:"items"`
					} `json:"data"`
				}
				if altErr := json.Unmarshal(bodyBytes, &alt); altErr == nil && len(alt.Data.Items) > 0 {
					for _, d := range alt.Data.Items {
						id := d.ID
						if id == "" {
							id = d.DeviceID
						}
						name := d.Name
						if name == "" {
							name = d.DeviceName
						}
						ip := d.IPAddress
						if ip == "" {
							ip = d.IP
						}
						if id != "" || name != "" {
							key := id
							if key == "" {
								key = name
							}
							deviceMap[key] = model.DeviceItem{
								ID:        id,
								Name:      name,
								IPAddress: ip,
								Status:    d.Status,
							}
						}
					}
				}
				break
			}
		}

		// If devices were successfully retrieved from first endpoint, skip secondary device endpoint
		if len(deviceMap) > 0 {
			break
		}
	}

	// 2. Discovery Fallback from recent access records (7 days lookback with pagination)
	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -7)
	recPage := 1
	recLimit := 1000
	const maxRecPages = 5 // Scan up to 5,000 records for device discovery

	for recPage <= maxRecPages {
		params := url.Values{}
		params.Set("startDateTime", sevenDaysAgo.Format(time.RFC3339))
		params.Set("endDateTime", now.Format(time.RFC3339))
		params.Set("page", strconv.Itoa(recPage))
		params.Set("limit", strconv.Itoa(recLimit))

		recURL := fmt.Sprintf("%s/api/access-records/?%s", c.cfg.AppAPIURL, params.Encode())
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, recURL, nil)
		if err != nil {
			break
		}

		authHeader, _ := c.GetAuthToken(ctx)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			break
		}

		if resp.StatusCode == http.StatusOK {
			bodyBytes, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				break
			}

			var recRes struct {
				Data []struct {
					AccessControlDeviceID   string `json:"accessControlDeviceId"`
					AccessControlDeviceName string `json:"accessControlDeviceName"`
				} `json:"data"`
				Page struct {
					TotalPage int `json:"totalPage"`
				} `json:"page"`
				Pagination struct {
					TotalPage int `json:"totalPage"`
				} `json:"pagination"`
			}

			if err := json.Unmarshal(bodyBytes, &recRes); err == nil && len(recRes.Data) > 0 {
				for _, r := range recRes.Data {
					id := r.AccessControlDeviceID
					name := r.AccessControlDeviceName
					if id != "" || name != "" {
						key := id
						if key == "" {
							key = name
						}
						if existing, exists := deviceMap[key]; exists {
							if existing.Name == "" && name != "" {
								existing.Name = name
								deviceMap[key] = existing
							}
						} else {
							deviceMap[key] = model.DeviceItem{
								ID:   id,
								Name: name,
							}
						}
					}
				}

				totalPage := recRes.Page.TotalPage
				if totalPage == 0 {
					totalPage = recRes.Pagination.TotalPage
				}

				if totalPage == 0 || recPage >= totalPage || len(recRes.Data) < recLimit {
					break
				}
				recPage++
			} else {
				break
			}
		} else {
			resp.Body.Close()
			break
		}
	}

	var devices []model.DeviceItem
	for _, dev := range deviceMap {
		devices = append(devices, dev)
	}

	return devices, nil
}
