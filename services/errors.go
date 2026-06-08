package services

import "errors"

// Business-layer sentinel errors.
var (
	ErrInvalidQuantity = errors.New("invalid quantity")
	ErrInvalidInput    = errors.New("invalid input")
	ErrCartEmpty       = errors.New("cart is empty")
)
