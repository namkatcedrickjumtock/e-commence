package persistence

import (
	"errors"
	"fmt"
)

// Persistence-layer sentinel errors.
// Each error is defined here and translated from raw database/provider errors
// at the boundary — no layer above persistence ever sees sql.ErrNoRows or a
// provider-specific error type.
var (
	ErrProductNotFound     = errors.New("product not found")
	ErrOrderNotFound       = errors.New("order not found")
	ErrProductOutOfStock   = errors.New("product out of stock")
	ErrDatabaseUnavailable = errors.New("database unavailable")
)

// PaymentError is the domain-level payment failure type.
// The persistence layer translates all provider-specific errors into this type —
// no Stripe codes, request IDs, or charge references ever reach the caller.
type PaymentError struct {
	Code      string
	Retryable bool
}

func (e *PaymentError) Error() string {
	return fmt.Sprintf("payment failed: %s", e.Code)
}

// Is supports errors.Is matching. A target with an empty Code matches any PaymentError.
func (e *PaymentError) Is(target error) bool {
	t, ok := target.(*PaymentError)
	if !ok {
		return false
	}
	return t.Code == "" || e.Code == t.Code
}
