package api

import "errors"

var (
	ErrInvalidJSON      = errors.New("invalid json")
	ErrMissingField     = errors.New("missing field")
	ErrInvalidProductID = errors.New("invalid product id")
	ErrInvalidOrderID   = errors.New("invalid order id")
)
