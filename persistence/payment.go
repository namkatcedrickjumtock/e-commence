package persistence

import "context"

type PaymentMode string

const (
	ModeOK           PaymentMode = "ok"
	ModeCardDeclined PaymentMode = "card_declined"
)

// StripeProvider simulates the Stripe payment API.
// Charge translates provider outcomes to domain PaymentErrors at the boundary —
// no Stripe-specific types, codes, or request IDs reach the caller.
type StripeProvider struct {
	mode PaymentMode
}

func NewStripeProvider(mode PaymentMode) *StripeProvider {
	return &StripeProvider{mode: mode}
}

func (p *StripeProvider) SetMode(mode PaymentMode) {
	p.mode = mode
}

func (p *StripeProvider) Charge(_ context.Context, _ int) error {
	if p.mode == ModeCardDeclined {
		return &PaymentError{Code: "payment_declined", Retryable: false}
	}
	return nil
}

func ParsePaymentMode(s string) PaymentMode {
	if s == "card_declined" {
		return ModeCardDeclined
	}
	return ModeOK
}
