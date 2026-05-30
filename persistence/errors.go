package persistence

import "errors"

// Persistence-layer sentinel errors.
// NOTE (bad version): these are defined but postgres.go does NOT use them for
// sql.ErrNoRows translation. Raw database errors propagate upward instead.
// The sentinels exist but are not wired into the error flow — callers that
// check for ErrProductNotFound via errors.Is will never match.
var (
	ErrProductOutOfStock    = errors.New("product out of stock")
	ErrDuplicateCartItem    = errors.New("duplicate cart item")
	ErrDatabaseUnavailable  = errors.New("database unavailable")
)
