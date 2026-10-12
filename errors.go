package aut2fa

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// ErrInvalidResponse identifies malformed or incomplete successful responses.
var ErrInvalidResponse = errors.New("aut2fa: invalid API response")

// ApiError preserves an HTTP rejection and its RFC 9457 problem details.
// RetryAfter is the raw Retry-After header, which may be seconds or an HTTP date.
type ApiError struct {
	Status     int
	Code       string
	Detail     string
	Errors     map[string]string
	RetryAfter string
}

func (err *ApiError) Error() string {
	return fmt.Sprintf("aut2fa: HTTP %d (%s): %s", err.Status, err.Code, err.Detail)
}

func apiError(status int, retryAfter string, body []byte) *ApiError {
	result := &ApiError{Status: status, Code: "http_error", Detail: http.StatusText(status), RetryAfter: retryAfter}
	var problem struct {
		Code   string            `json:"code"`
		Detail string            `json:"detail"`
		Errors map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &problem); err != nil || strings.TrimSpace(problem.Code) == "" {
		return result
	}
	result.Code = problem.Code
	result.Detail = problem.Detail
	result.Errors = problem.Errors
	return result
}
