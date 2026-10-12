package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/jhowbhz/aut2fa-go"
)

func main() {
	client, err := aut2fa.New(os.Getenv("AUT2FA_API_KEY"), aut2fa.WithTimeout(10*time.Second))
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	send, err := client.Send(ctx, aut2fa.SendParams{
		To: os.Getenv("AUT2FA_TO"), Code: "123456", Locale: "pt-BR",
		IdempotencyKey: os.Getenv("AUT2FA_IDEMPOTENCY_KEY"),
	})
	if err != nil {
		fail(err)
	}
	fmt.Println("send:", send.ID, send.Status)

	status, err := client.Status(ctx, send.ID)
	if err != nil {
		fail(err)
	}
	fmt.Println("status:", status.Status)

	verification, err := client.Verify(ctx, send.ID, "123456")
	if err != nil {
		fail(err)
	}
	fmt.Println("valid:", verification.Valid, "attempts left:", verification.AttemptsLeft)
}

func fail(err error) {
	var problem *aut2fa.ApiError
	if errors.As(err, &problem) {
		log.Fatalf("API rejected request: status=%d code=%s detail=%s retry_after=%s",
			problem.Status, problem.Code, problem.Detail, problem.RetryAfter)
	}
	log.Fatal(err)
}
