// Package aut2fa sends and verifies 2FA codes through the public aut2fa API.
package aut2fa

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client uses one immutable application key and configuration across requests.
// Create it with New; it can be shared by concurrent goroutines.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// New validates credentials and applies options before creating the client.
func New(apiKey string, options ...Option) (*Client, error) {
	if strings.TrimSpace(apiKey) == "" || containsControl(apiKey) {
		return nil, errors.New("aut2fa: API key must be nonempty and contain no control characters")
	}
	cfg := configuration{baseURL: defaultBaseURL, timeout: defaultTimeout, httpClient: &http.Client{}}
	for _, option := range options {
		if option == nil {
			return nil, errors.New("aut2fa: option must not be nil")
		}
		if err := option(&cfg); err != nil {
			return nil, err
		}
	}
	httpClient := *cfg.httpClient
	httpClient.Timeout = cfg.timeout
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	transport := httpClient.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	if standard, ok := transport.(*http.Transport); ok {
		copy := standard.Clone()
		// Reused connections can trigger net/http's transparent request retries.
		copy.DisableKeepAlives = true
		transport = copy
	}
	httpClient.Transport = transport
	return &Client{apiKey: apiKey, baseURL: cfg.baseURL, httpClient: &httpClient}, nil
}

// Send creates a delivery. An idempotency key is sent only as an HTTP header.
func (c *Client) Send(ctx context.Context, params SendParams) (*SendResponse, error) {
	if strings.TrimSpace(params.To) == "" {
		return nil, errors.New("aut2fa: destination must be nonempty")
	}
	if len(params.IdempotencyKey) > maxIdempotencyKeyBytes || containsControl(params.IdempotencyKey) {
		return nil, errors.New("aut2fa: idempotency key must contain at most 255 bytes and no control characters")
	}
	body, err := c.request(ctx, http.MethodPost, "/v1/send", params, params.IdempotencyKey)
	if err != nil {
		return nil, err
	}
	var response SendResponse
	if err := decodeResponse(body, &response, "id", "status"); err != nil {
		return nil, err
	}
	return &response, nil
}

// Status fetches the current delivery and verification state.
func (c *Client) Status(ctx context.Context, id string) (*SendResponse, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	body, err := c.request(ctx, http.MethodGet, "/v1/send/"+id+"/status", nil, "")
	if err != nil {
		return nil, err
	}
	var response SendResponse
	if err := decodeResponse(body, &response, "id", "status"); err != nil {
		return nil, err
	}
	return &response, nil
}

// Verify checks a code once; Valid=false is a normal successful result.
func (c *Client) Verify(ctx context.Context, id, code string) (*VerifyResponse, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(code) == "" {
		return nil, errors.New("aut2fa: verification code must be nonempty")
	}
	payload := struct {
		Code string `json:"code"`
	}{Code: code}
	body, err := c.request(ctx, http.MethodPost, "/v1/send/"+id+"/verify", payload, "")
	if err != nil {
		return nil, err
	}
	var response VerifyResponse
	if err := decodeResponse(body, &response, "valid", "verification", "attempts_left"); err != nil {
		return nil, err
	}
	return &response, nil
}

func (c *Client) request(ctx context.Context, method, path string, payload any, idempotencyKey string) ([]byte, error) {
	var reader io.Reader
	if payload != nil {
		body, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("aut2fa: encode request: %w", err)
		}
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	// Idempotency-Key makes replayable POSTs eligible for retries in net/http.
	req.GetBody = nil
	req.Close = true
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	body, readErr := io.ReadAll(response.Body)
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, apiError(response.StatusCode, response.Header.Get("Retry-After"), body)
	}
	if readErr != nil {
		return nil, fmt.Errorf("aut2fa: read response: %w", readErr)
	}
	return body, nil
}

func decodeResponse(body []byte, target any, required ...string) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return ErrInvalidResponse
	}
	for _, field := range required {
		value, exists := fields[field]
		if !exists || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return ErrInvalidResponse
		}
	}
	if err := json.Unmarshal(body, target); err != nil {
		return ErrInvalidResponse
	}
	switch response := target.(type) {
	case *SendResponse:
		if strings.TrimSpace(response.ID) == "" || strings.TrimSpace(response.Status) == "" {
			return ErrInvalidResponse
		}
	case *VerifyResponse:
		if strings.TrimSpace(response.Verification) == "" || response.AttemptsLeft < 0 {
			return ErrInvalidResponse
		}
	}
	return nil
}

func validateID(id string) error {
	if id == "" {
		return errors.New("aut2fa: send ID must be nonempty")
	}
	for _, char := range id {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return errors.New("aut2fa: send ID must contain only ASCII letters, digits, underscore or hyphen")
	}
	return nil
}
