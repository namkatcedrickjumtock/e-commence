package persistence

import (
	"crypto/rand"
	"encoding/hex"
)

// Domain types owned by the persistence layer.

type Product struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	PriceCents int    `json:"price_cents"`
	Stock      int    `json:"stock"`
}

type CartItem struct {
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
}

type Order struct {
	ID         string     `json:"id"`
	TotalCents int        `json:"total_cents"`
	Items      []CartItem `json:"items"`
}

func newProductID() string { return "p_" + randomHex() }
func newOrderID() string   { return "o_" + randomHex() }

func randomHex() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
