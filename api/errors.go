package api

import "errors"

// Presentation-owned errors: request/HTTP concerns only.
var (
	ErrInvalidJSON      = errors.New("invalid json")
	ErrMissingField     = errors.New("missing field")
	ErrInvalidProductID = errors.New("invalid product id")
)

