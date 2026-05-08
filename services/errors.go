package services

import "errors"

// Business-layer sentinel errors.
// These originate from business-rule validation and use-case orchestration.
// Data-access errors (product not found, out of stock, etc.) are owned by
// the persistence layer; only purely business rules live here.
var (
	ErrDuplicateProduct = errors.New("product with this name already exists")
	ErrInvalidQuantity  = errors.New("invalid quantity")
	ErrInvalidInput     = errors.New("invalid input")
	ErrCartEmpty        = errors.New("cart is empty")
	ErrPaymentDeclined  = errors.New("payment declined")
)
