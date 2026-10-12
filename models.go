package aut2fa

import "time"

// SendParams carries a destination and optional code, locale and idempotency key.
// An empty Code lets the API generate a code; an empty Locale uses its default.
type SendParams struct {
	To             string `json:"to"`
	Code           string `json:"code,omitempty"`
	Locale         string `json:"locale,omitempty"`
	IdempotencyKey string `json:"-"`
}

// SendResponse preserves JSON nulls as nil pointers and timestamps as time.Time.
// State strings remain open to values added by future API versions.
type SendResponse struct {
	ID            string     `json:"id"`
	Status        string     `json:"status"`
	Verification  string     `json:"verification"`
	Mode          string     `json:"mode"`
	Channel       *string    `json:"channel"`
	From          *string    `json:"from"`
	ExpiresAt     time.Time  `json:"expires_at"`
	CreatedAt     time.Time  `json:"created_at"`
	SentAt        *time.Time `json:"sent_at"`
	DeliveredAt   *time.Time `json:"delivered_at"`
	ReadAt        *time.Time `json:"read_at"`
	VerifiedAt    *time.Time `json:"verified_at"`
	FailedAt      *time.Time `json:"failed_at"`
	FailureReason *string    `json:"failure_reason"`
}

// VerifyResponse records one verification attempt, including an invalid code.
type VerifyResponse struct {
	Valid        bool   `json:"valid"`
	Verification string `json:"verification"`
	AttemptsLeft int    `json:"attempts_left"`
}
