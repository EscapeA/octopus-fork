package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/lingyuins/octopus/internal/model"
	"github.com/lingyuins/octopus/internal/utils/crypto"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

// apiResponse is the generic envelope returned by One API / New API compatible backends.
// Fields are intentionally interface{} to handle divergent server implementations.
type apiResponse struct {
	Success *bool       `json:"success,omitempty"`
	Code    *int        `json:"code,omitempty"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// doRequest performs an HTTP request against a remote site and returns the raw
// response body. The caller is responsible for closing the body.
func doRequest(ctx context.Context, method, url string, body io.Reader, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return httpClient.Do(req)
}

// buildAuthHeaders returns the Authorization header value for a remote site.
func buildAuthHeaders(site *model.RemoteSite) (map[string]string, error) {
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if site.AuthType == model.AuthTypeNone {
		return headers, nil
	}
	token, err := crypto.Decrypt(site.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt access token: %w", err)
	}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}
	return headers, nil
}

// baseURL normalizes the site base URL (strips trailing slash).
func baseURL(site *model.RemoteSite) string {
	return strings.TrimRight(site.BaseURL, "/")
}

// FetchJSON performs a JSON API call and unmarshals the data field into dest.
func FetchJSON[T any](ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}) (T, error) {
	var zero T

	headers, err := buildAuthHeaders(site)
	if err != nil {
		return zero, err
	}

	var body io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return zero, fmt.Errorf("marshal request body: %w", err)
		}
		body = strings.NewReader(string(b))
	}

	url := baseURL(site) + endpoint
	resp, err := doRequest(ctx, method, url, body, headers)
	if err != nil {
		return zero, fmt.Errorf("request %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return zero, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, endpoint, truncate(string(respBody), 200))
	}

	var envelope apiResponse
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return zero, fmt.Errorf("unmarshal response from %s: %w", endpoint, err)
	}

	if envelope.Success != nil && !*envelope.Success {
		return zero, fmt.Errorf("API error from %s: %s", endpoint, envelope.Message)
	}
	if envelope.Code != nil && *envelope.Code != 200 {
		return zero, fmt.Errorf("API error from %s (code %d): %s", endpoint, *envelope.Code, envelope.Message)
	}

	if envelope.Data == nil {
		return zero, nil
	}

	dataBytes, err := json.Marshal(envelope.Data)
	if err != nil {
		return zero, fmt.Errorf("re-marshal data: %w", err)
	}

	var result T
	if err := json.Unmarshal(dataBytes, &result); err != nil {
		return zero, fmt.Errorf("unmarshal data into %T: %w", result, err)
	}
	return result, nil
}

// fetchRawJSON is like fetchJSON but returns the raw data bytes without
// unmarshaling into a specific type.
func fetchRawJSON(ctx context.Context, site *model.RemoteSite, method, endpoint string, reqBody interface{}) (json.RawMessage, error) {
	headers, err := buildAuthHeaders(site)
	if err != nil {
		return nil, err
	}

	var body io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		body = strings.NewReader(string(b))
	}

	url := baseURL(site) + endpoint
	resp, err := doRequest(ctx, method, url, body, headers)
	if err != nil {
		return nil, fmt.Errorf("request %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from %s: %s", resp.StatusCode, endpoint, truncate(string(respBody), 200))
	}

	var envelope apiResponse
	if err := json.Unmarshal(respBody, &envelope); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if envelope.Success != nil && !*envelope.Success {
		return nil, fmt.Errorf("API error from %s: %s", endpoint, envelope.Message)
	}
	if envelope.Code != nil && *envelope.Code != 200 {
		return nil, fmt.Errorf("API error from %s (code %d): %s", endpoint, *envelope.Code, envelope.Message)
	}

	if envelope.Data == nil {
		return nil, nil
	}

	return json.Marshal(envelope.Data)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
