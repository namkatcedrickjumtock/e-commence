package persistence

// Domain types owned by the persistence layer.
// The business layer (services) imports these from here rather than
// defining its own copies, keeping the dependency graph one-directional:
//   services → persistence → sqlc
// This avoids the import cycle that would arise if both layers
// defined their own versions of the same types.

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
