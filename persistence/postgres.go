package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/namkatcedrickjumtock/e-commence/persistence/sqlc"
)

type PostgresRepo struct {
	db *sql.DB
	q  *sqlc.Queries
}

func NewPostgresRepo(db *sql.DB) *PostgresRepo {
	return &PostgresRepo{db: db, q: sqlc.New(db)}
}

// Get fetches a single product row from the products table by ID.
func (r *PostgresRepo) Get(ctx context.Context, id string) (*Product, error) {
	p, err := r.q.GetProduct(ctx, id)
	if err != nil {
		// Scenario 1: wraps without translating — sql.ErrNoRows propagates to every caller.
		return nil, fmt.Errorf("GetProduct failed: %w", err)
	}
	return &Product{
		ID:         p.ID,
		Name:       p.Name,
		PriceCents: int(p.PriceCents),
		Stock:      int(p.Stock),
	}, nil
}

// GetByName fetches a product row by name using a raw query — used to check for duplicates.
func (r *PostgresRepo) GetByName(ctx context.Context, name string) (*Product, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, price_cents, stock FROM products WHERE name = $1`, name)

	var p sqlc.Product
	if err := row.Scan(&p.ID, &p.Name, &p.PriceCents, &p.Stock); err != nil {
		return nil, fmt.Errorf("GetByName failed: %w", err)
	}
	return &Product{
		ID:         p.ID,
		Name:       p.Name,
		PriceCents: int(p.PriceCents),
		Stock:      int(p.Stock),
	}, nil
}

// Create inserts a new product row into the products table.
func (r *PostgresRepo) Create(ctx context.Context, product Product) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO products (id, name, price_cents, stock) VALUES ($1, $2, $3, $4)`,
		product.ID, product.Name, product.PriceCents, product.Stock)
	if err != nil {
		return fmt.Errorf("Create failed: %w", err)
	}
	return nil
}

// Reserve decrements the stock of a product by the given quantity.
func (r *PostgresRepo) Reserve(ctx context.Context, productID string, quantity int) error {
	err := r.q.ReserveStock(ctx, sqlc.ReserveStockParams{
		ID:    productID,
		Stock: int32(quantity),
	})
	if err != nil {
		return fmt.Errorf("Reserve failed: %w", err)
	}
	return nil
}

// AddItem inserts a cart item row linking a product ID and quantity.
func (r *PostgresRepo) AddItem(ctx context.Context, item CartItem) error {
	err := r.q.InsertCartItem(ctx, sqlc.InsertCartItemParams{
		ProductID: item.ProductID,
		Quantity:  int32(item.Quantity),
	})
	if err != nil {
		return fmt.Errorf("AddItem failed: %w", err)
	}
	return nil
}

// Items returns all rows from the cart_items table.
func (r *PostgresRepo) Items(ctx context.Context) ([]CartItem, error) {
	rows, err := r.q.ListCartItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("ListCartItems failed: %w", err)
	}
	items := make([]CartItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, CartItem{
			ProductID: row.ProductID,
			Quantity:  int(row.Quantity),
		})
	}
	return items, nil
}

// Clear deletes all rows from the cart_items table.
func (r *PostgresRepo) Clear(ctx context.Context) error {
	if err := r.q.ClearCart(ctx); err != nil {
		return fmt.Errorf("ClearCart failed: %w", err)
	}
	return nil
}

// CreateOrder inserts an order header row and all its line items in sequence.
func (r *PostgresRepo) CreateOrder(ctx context.Context, order Order) (*Order, error) {
	if err := r.q.CreateOrder(ctx, sqlc.CreateOrderParams{
		ID:         order.ID,
		TotalCents: int32(order.TotalCents),
	}); err != nil {
		return nil, fmt.Errorf("CreateOrder failed: %w", err)
	}
	for _, it := range order.Items {
		if err := r.q.AddOrderItem(ctx, sqlc.AddOrderItemParams{
			OrderID:   order.ID,
			ProductID: it.ProductID,
			Quantity:  int32(it.Quantity),
		}); err != nil {
			return nil, fmt.Errorf(
				"AddOrderItem failed order_id=%s product_id=%s: %w",
				order.ID, it.ProductID, err,
			)
		}
	}
	return &order, nil
}

// GetOrder fetches an order by ID, then loads its line items in a second query.
func (r *PostgresRepo) GetOrder(ctx context.Context, id string) (*Order, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, total_cents FROM orders WHERE id = $1`, id)

	var o sqlc.Order
	if err := row.Scan(&o.ID, &o.TotalCents); err != nil {
		// Scenario 1: three prefixes for one lookup — the chain starts here and grows at each layer.
		return nil, fmt.Errorf("GetOrder failed: %w", err)
	}

	items, err := r.orderItems(ctx, id)
	if err != nil {
		return nil, err
	}

	return &Order{
		ID:         o.ID,
		TotalCents: int(o.TotalCents),
		Items:      items,
	}, nil
}

func (r *PostgresRepo) orderItems(ctx context.Context, orderID string) ([]CartItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, quantity FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, fmt.Errorf("orderItems failed: %w", err)
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var oi CartItem
		if err := rows.Scan(&oi.ProductID, &oi.Quantity); err != nil {
			return nil, fmt.Errorf("orderItems scan failed: %w", err)
		}
		items = append(items, oi)
	}
	return items, rows.Err()
}

// Reset truncates all tables in dependency order — used by the demo reset endpoint.
func (r *PostgresRepo) Reset(ctx context.Context) error {
	for _, stmt := range []string{
		"DELETE FROM order_items",
		"DELETE FROM orders",
		"DELETE FROM cart_items",
		"DELETE FROM products",
	} {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("reset failed on %q: %w", stmt, err)
		}
	}
	return nil
}
