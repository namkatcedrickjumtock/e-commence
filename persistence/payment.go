package persistence

import (
	"context"
	"fmt"
	"time"
)

// FlutterwaveError leaks raw internal payment-provider state directly to callers.
// No abstraction layer. Internal codes, regions, transaction references, and retry
// hints all reach whoever catches this error — including the HTTP response body.
//
// DEMO BAD PATTERN: Provider internals are part of the public error contract.
// Any layer that receives this learns about our payment processor, the EU region
// topology, the transaction reference schema, and internal retry policy.
type FlutterwaveError struct {
	Code      string // internal provider error code, e.g. "FW-9082"
	Region    string // deployment region, e.g. "eu-west"
	Retryable bool
	TxRef     string // internal transaction reference
	Message   string // raw provider message, may include acquirer codes
}

func (e *FlutterwaveError) Error() string {
	return fmt.Sprintf(
		"flutterwave error %s region=%s retryable=%v tx_ref=%s: %s",
		e.Code, e.Region, e.Retryable, e.TxRef, e.Message,
	)
}

type PaymentMode string

const (
	ModeOK              PaymentMode = "ok"
	ModeCardDeclined    PaymentMode = "card_declined"
	ModeProcessingError PaymentMode = "processing_error"
	ModeRateLimit       PaymentMode = "rate_limit"
	ModeAPITimeout      PaymentMode = "api_timeout"
)

// FlutterwaveProvider simulates the Flutterwave payment API.
// Unlike a well-designed provider wrapper, this returns its raw internal
// error struct directly — no translation, no sentinel errors, no abstraction.
type FlutterwaveProvider struct {
	mode PaymentMode
}

func NewFlutterwaveProvider(mode PaymentMode) *FlutterwaveProvider {
	return &FlutterwaveProvider{mode: mode}
}

// SetMode allows runtime mutation of the payment mode.
// DEMO BAD PATTERN: mutable global-ish state on an infrastructure object,
// exposed for a demo endpoint that bypasses the service layer entirely.
func (p *FlutterwaveProvider) SetMode(mode PaymentMode) {
	p.mode = mode
}

// Charge processes a payment. On failure it returns *FlutterwaveError directly —
// the caller receives raw provider internals with no translation or wrapping.
func (p *FlutterwaveProvider) Charge(ctx context.Context, amountCents int) error {
	switch p.mode {
	case ModeCardDeclined:
		return &FlutterwaveError{
			Code:      "FW-9082",
			Region:    "eu-west",
			Retryable: false,
			TxRef:     "FW-TXN-84712947",
			Message:   "CARD_DECLINED: insufficient_funds, issuer_code=05, network=VISA_EU",
		}
	case ModeProcessingError:
		return &FlutterwaveError{
			Code:      "FW-5004",
			Region:    "eu-west",
			Retryable: true,
			TxRef:     "FW-TXN-84712948",
			Message:   "PROCESSOR_ERROR: upstream_acquirer=worldline_eu status=TIMEOUT_ON_AUTH",
		}
	case ModeRateLimit:
		return &FlutterwaveError{
			Code:      "FW-4290",
			Region:    "eu-west",
			Retryable: true,
			TxRef:     "",
			Message:   "RATE_LIMIT_EXCEEDED: merchant_id=MW-4721 requests_per_min=100 retry_after=30s",
		}
	case ModeAPITimeout:
		select {
		case <-ctx.Done():
			return &FlutterwaveError{
				Code:      "FW-5001",
				Region:    "eu-west",
				Retryable: true,
				TxRef:     "FW-TXN-84712949",
				Message:   "GATEWAY_TIMEOUT: upstream_provider=mastercard_eu_gateway retry_after=30s connection_pool=exhausted",
			}
		case <-time.After(500 * time.Millisecond):
			return &FlutterwaveError{
				Code:      "FW-5001",
				Region:    "eu-west",
				Retryable: true,
				TxRef:     "FW-TXN-84712949",
				Message:   "GATEWAY_TIMEOUT: upstream_provider=mastercard_eu_gateway retry_after=30s connection_pool=exhausted",
			}
		}
	}
	return nil
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
