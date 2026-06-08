package persistence

import (
	"context"
	"fmt"
)

// StripeError leaks raw internal payment-provider state directly to callers.
// No abstraction layer — internal codes, request IDs, charge references, and
// decline codes all reach whoever catches this error, including the HTTP response.
//
// DEMO BAD PATTERN: Provider internals are part of the public error contract.
// Any layer that receives this learns our payment processor, the request
// tracing ID, the charge reference, and the issuer's decline reason.
type StripeError struct {
	RequestID   string // Stripe request tracing ID, e.g. "req_7NdMtHtVhh5i2J"
	Type        string // provider error category, e.g. "card_error"
	Code        string // machine-readable code, e.g. "card_declined"
	DeclineCode string // issuer decline reason, e.g. "insufficient_funds"
	ChargeID    string // internal charge reference
	Message     string // raw provider message
}

func (e *StripeError) Error() string {
	return fmt.Sprintf(
		"stripe error request_id=%s type=%s code=%s decline_code=%s charge=%s: %s",
		e.RequestID, e.Type, e.Code, e.DeclineCode, e.ChargeID, e.Message,
	)
}

type PaymentMode string

const (
	ModeOK           PaymentMode = "ok"
	ModeCardDeclined PaymentMode = "card_declined"
)

// StripeProvider simulates the Stripe payment API.
// Unlike a well-designed provider wrapper, this returns its raw internal
// error struct directly — no translation, no sentinel errors, no abstraction.
type StripeProvider struct {
	mode PaymentMode
}

func NewStripeProvider(mode PaymentMode) *StripeProvider {
	return &StripeProvider{mode: mode}
}

func (p *StripeProvider) SetMode(mode PaymentMode) {
	p.mode = mode
}

// Charge processes a payment. On failure it returns *StripeError directly —
// the caller receives raw provider internals with no translation or wrapping.
func (p *StripeProvider) Charge(_ context.Context, _ int) error {
	if p.mode == ModeCardDeclined {
		return &StripeError{
			RequestID:   "req_7NdMtHtVhh5i2J",
			Type:        "card_error",
			Code:        "card_declined",
			DeclineCode: "insufficient_funds",
			ChargeID:    "ch_3RJXsD2eZvKYlo2C0B5X3Y7z",
			Message:     "Your card has insufficient funds.",
		}
	}
	return nil
}

func ParsePaymentMode(s string) PaymentMode {
	if s == "card_declined" {
		return ModeCardDeclined
	}
	return ModeOK
}
