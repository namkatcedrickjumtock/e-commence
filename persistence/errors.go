package persistence

import "errors"

// Sentinel errors owned by the persistence layer.
// These originate from data-access operations (DB, external APIs, etc.)
// and flow up through the business layer to the presentation layer
// for HTTP status mapping. The business layer may translate some of
// these into higher-level domain errors where appropriate.
var (
	ErrProductNotFound       = errors.New("product not found")
	ErrProductOutOfStock     = errors.New("product out of stock")
	ErrDuplicateCartItem     = errors.New("duplicate cart item")
	ErrOrderNotFound         = errors.New("order not found")
	ErrDatabaseUnavailable   = errors.New("database unavailable")
	ErrPaymentProviderTimeout = errors.New("payment provider timeout")
)
