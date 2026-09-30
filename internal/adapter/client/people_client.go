package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/putteror/sea-value/internal/core/model"
)

// UploadPeopleExcel uploads an Excel file directly to CentAccess (POST /api/people/import/:personType)
func (c *CentAccessAPIClient) UploadPeopleExcel(ctx context.Context, filePath string, personType string) (*model.ImportResult, error) {
	if personType == "" {
		personType = "EMPLOYEE"
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open excel file %s: %w", filePath, err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file to multipart: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	reqURL := fmt.Sprintf("%s/api/people/import/%s", c.cfg.AppAPIURL, personType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	authHeader, err := c.GetAuthToken(ctx)
	if err == nil && authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("import api returned status %d: %s", resp.StatusCode, string(body))
	}

	return &model.ImportResult{
		SuccessCount: 1,
		Message:      "Excel uploaded and processed successfully",
	}, nil
}

// ImportPeople imports people individual/batch fallback (POST /api/people)
func (c *CentAccessAPIClient) ImportPeople(ctx context.Context, people []model.PeopleImportRow) (*model.ImportResult, error) {
	successCount := 0
	failedCount := 0
	var errorMsgs []string

	authHeader, _ := c.GetAuthToken(ctx)

	for _, p := range people {
		reqBody := model.CreatePersonRequest{
			FirstName:    p.FirstName,
			LastName:     p.LastName,
			PersonType:   "EMPLOYEE",
			PersonCode:   p.PersonCode,
			Department:   p.Department,
			JobPosition:  p.Position,
			Email:        p.Email,
			MobileNumber: p.Phone,
		}
		if p.CardNumber != "" {
			reqBody.AccessCardNumbers = []string{p.CardNumber}
		}

		payloadBytes, _ := json.Marshal(reqBody)
		reqURL := fmt.Sprintf("%s/api/people", c.cfg.AppAPIURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(payloadBytes))
		if err != nil {
			failedCount++
			errorMsgs = append(errorMsgs, fmt.Sprintf("%s %s: %v", p.FirstName, p.LastName, err))
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil || (resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated) {
			failedCount++
			if err != nil {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s %s: %v", p.FirstName, p.LastName, err))
			} else {
				errorMsgs = append(errorMsgs, fmt.Sprintf("%s %s: status %d", p.FirstName, p.LastName, resp.StatusCode))
				resp.Body.Close()
			}
			continue
		}
		resp.Body.Close()
		successCount++
	}

	return &model.ImportResult{
		TotalProcessed: len(people),
		SuccessCount:   successCount,
		FailedCount:    failedCount,
		Errors:         errorMsgs,
	}, nil
}
