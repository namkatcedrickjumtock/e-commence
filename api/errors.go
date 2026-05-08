package api

import "errors"

// Presentation-layer sentinel errors.
// These cover request parsing, validation, and other HTTP-specific concerns.
// The business and persistence layers never see these errors.
var (
	ErrInvalidJSON       = errors.New("invalid json")
	ErrMissingField      = errors.New("missing field")
	ErrInvalidProductID  = errors.New("invalid product id")
	ErrInvalidOrderID    = errors.New("invalid order id")
	ErrInvalidName       = errors.New("invalid product name")
	ErrInvalidPrice      = errors.New("invalid price cents")
	ErrInvalidStock      = errors.New("invalid stock value")
)
