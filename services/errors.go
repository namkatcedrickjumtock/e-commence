package services

import "errors"

// Business-layer sentinel errors.
// NOTE (bad version): ErrPaymentDeclined has been removed.
// Payment failures now propagate raw StripeError structs directly to
// the presentation layer — provider internals leak all the way to the client.
var (
	ErrDuplicateProduct = errors.New("product with this name already exists")
	ErrInvalidQuantity  = errors.New("invalid quantity")
	ErrInvalidInput     = errors.New("invalid input")
	ErrCartEmpty        = errors.New("cart is empty")
)
