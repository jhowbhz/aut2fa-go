package aut2fa

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	defaultBaseURL         = "https://api.aut2fa.com"
	defaultTimeout         = 10 * time.Second
	maxIdempotencyKeyBytes = 255
)

type configuration struct {
	baseURL    string
	timeout    time.Duration
	httpClient *http.Client
}

// Option configures a Client before it is created.
type Option func(*configuration) error

// WithBaseURL sets an absolute HTTP(S) URL, optionally with a path prefix.
// Credentials, queries and fragments are not accepted.
func WithBaseURL(baseURL string) Option {
	return func(cfg *configuration) error {
		parsed, err := url.Parse(baseURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" ||
			parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || strings.Contains(baseURL, "#") || parsed.Opaque != "" {
			return errors.New("aut2fa: base URL must be an absolute HTTP(S) URL without credentials, query or fragment")
		}
		if port := parsed.Port(); port != "" {
			number, err := strconv.Atoi(port)
			if err != nil || number < 1 || number > 65535 {
				return errors.New("aut2fa: base URL port must be between 1 and 65535")
			}
		}
		cfg.baseURL = strings.TrimRight(parsed.String(), "/")
		return nil
	}
}

// WithTimeout sets the total request timeout, including reading the response.
// The duration must be positive; the default is ten seconds.
func WithTimeout(timeout time.Duration) Option {
	return func(cfg *configuration) error {
		if timeout <= 0 {
			return errors.New("aut2fa: timeout must be positive")
		}
		cfg.timeout = timeout
		return nil
	}
}

// WithHTTPClient preserves an injected client's transport and cookie jar.
// The SDK copies the client and enforces its own timeout and redirect policy.
// Standard transports are cloned with keep-alives disabled to prevent retries.
func WithHTTPClient(client *http.Client) Option {
	return func(cfg *configuration) error {
		if client == nil {
			return errors.New("aut2fa: HTTP client must not be nil")
		}
		cfg.httpClient = client
		return nil
	}
}

func containsControl(value string) bool {
	return strings.ContainsFunc(value, unicode.IsControl)
}
