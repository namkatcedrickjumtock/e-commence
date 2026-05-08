package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Sentinel errors owned by the persistence (infrastructure) layer.
// The business layer maps some of these to domain errors; others
// pass through unmodified to the presentation layer.
var (
	ErrStripeCardDeclined    = errors.New("stripe: card was declined")
	ErrStripeProcessingError = errors.New("stripe: processing error")
	ErrStripeRateLimit       = errors.New("stripe: too many requests")
	ErrStripeAPIError        = errors.New("stripe: api error")
)

type PaymentMode string

const (
	ModeOK              PaymentMode = "ok"
	ModeCardDeclined    PaymentMode = "card_declined"
	ModeProcessingError PaymentMode = "processing_error"
	ModeRateLimit       PaymentMode = "rate_limit"
	ModeAPITimeout      PaymentMode = "api_timeout"
)

// StripeProvider simulates the Stripe payment API.
// Each mode triggers a different error path.
type StripeProvider struct {
	mode PaymentMode
}

func NewStripeProvider(mode PaymentMode) *StripeProvider {
	return &StripeProvider{mode: mode}
}

// Charge returns a sentinel error owned by the persistence layer,
// optionally wrapped with a human-readable detail string.
// The business layer's Checkout use-case translates card-declined
// into services.ErrPaymentDeclined; all other errors pass through.
func (p *StripeProvider) Charge(ctx context.Context, amountCents int) error {
	switch p.mode {
	case ModeCardDeclined:
		return fmt.Errorf("%w: your card does not support this type of purchase",
			ErrStripeCardDeclined)
	case ModeProcessingError:
		return fmt.Errorf("%w: an unexpected error occurred while processing your card",
			ErrStripeProcessingError)
	case ModeRateLimit:
		return fmt.Errorf("%w: rate limit exceeded, please try again later",
			ErrStripeRateLimit)
	case ModeAPITimeout:
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w: stripe api request timed out",
				ErrPaymentProviderTimeout)
		case <-time.After(500 * time.Millisecond):
			return fmt.Errorf("%w: stripe api did not respond in time",
				ErrPaymentProviderTimeout)
		}
	default:
		return nil
	}
}

func ParsePaymentMode(s string) PaymentMode {
	switch s {
	case "card_declined":
		return ModeCardDeclined
	case "processing_error":
		return ModeProcessingError
	case "rate_limit":
		return ModeRateLimit
	case "api_timeout":
		return ModeAPITimeout
	default:
		return ModeOK
	}
}

func IsStripeCardDeclined(err error) bool {
	return errors.Is(err, ErrStripeCardDeclined)
}

func IsStripeRateLimit(err error) bool {
	return errors.Is(err, ErrStripeRateLimit)
}
