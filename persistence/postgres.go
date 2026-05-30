package persistence

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/namkatcedrickjumtock/e-commence/persistence/sqlc"
)

// PostgresRepo wraps sqlc-generated queries.
//
// DEMO BAD PATTERNS:
//   - sql.ErrNoRows is NEVER translated into a sentinel error.
//     Raw database/sql errors propagate upward, coupling every caller to SQL.
//   - Every method wraps with 2-3 nested messages that repeat context
//     already present in the error itself ("database error: sql: ...").
//   - The caller receives opaque chains like:
//     "repository: GetProduct query failed: database error: sql: no rows in result set"
//     and must either string-match or check sql.ErrNoRows directly.
type PostgresRepo struct {
	db *sql.DB
	q  *sqlc.Queries
}

func NewPostgresRepo(db *sql.DB) *PostgresRepo {
	return &PostgresRepo{db: db, q: sqlc.New(db)}
}

func (r *PostgresRepo) Get(ctx context.Context, id string) (Product, error) {
	p, err := r.q.GetProduct(ctx, id)
	if err != nil {
		// DEMO: no sql.ErrNoRows → ErrProductNotFound translation.
		// Raw sql error leaks up. Every caller must know about database/sql.
		return Product{}, fmt.Errorf("repository: GetProduct query failed: database error: %w", err)
	}
	return Product{
		ID:         p.ID,
		Name:       p.Name,
		PriceCents: int(p.PriceCents),
		Stock:      int(p.Stock),
	}, nil
}

func (r *PostgresRepo) GetByName(ctx context.Context, name string) (Product, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, price_cents, stock FROM products WHERE name = $1`, name)

	var p sqlc.Product
	if err := row.Scan(&p.ID, &p.Name, &p.PriceCents, &p.Stock); err != nil {
		// DEMO: raw scan error leaks — includes "sql: no rows in result set" verbatim.
		return Product{}, fmt.Errorf("repository: GetByName query failed: database error: %w", err)
	}
	return Product{
		ID:         p.ID,
		Name:       p.Name,
		PriceCents: int(p.PriceCents),
		Stock:      int(p.Stock),
	}, nil
}

func (r *PostgresRepo) List(ctx context.Context) ([]Product, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, price_cents, stock FROM products ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("repository: List query failed: database connection error: %w", err)
	}
	defer rows.Close()

	var out []Product
	for rows.Next() {
		var p sqlc.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.PriceCents, &p.Stock); err != nil {
			return nil, fmt.Errorf("repository: List scan failed: database row scan error: %w", err)
		}
		out = append(out, Product{
			ID:         p.ID,
			Name:       p.Name,
			PriceCents: int(p.PriceCents),
			Stock:      int(p.Stock),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: List iteration failed: database cursor error: %w", err)
	}
	return out, nil
}

func (r *PostgresRepo) Create(ctx context.Context, product Product) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO products (id, name, price_cents, stock) VALUES ($1, $2, $3, $4)`,
		product.ID, product.Name, product.PriceCents, product.Stock)
	if err != nil {
		return fmt.Errorf("repository: Create insert failed: database write error: %w", err)
	}
	return nil
}

func (r *PostgresRepo) Reserve(ctx context.Context, productID string, quantity int) error {
	err := r.q.ReserveStock(ctx, sqlc.ReserveStockParams{
		ID:    productID,
		Stock: int32(quantity),
	})
	if err != nil {
		return fmt.Errorf("repository: Reserve stock update failed: database write error: %w", err)
	}
	return nil
}

func (r *PostgresRepo) AddItem(ctx context.Context, item CartItem) error {
	err := r.q.InsertCartItem(ctx, sqlc.InsertCartItemParams{
		ProductID: item.ProductID,
		Quantity:  int32(item.Quantity),
	})
	if err != nil {
		// DEMO: raw postgres error leaks — includes pq-specific constraint violation text.
		return fmt.Errorf("repository layer: failed to add cart item: database error: %w", err)
	}
	return nil
}

func (r *PostgresRepo) Items(ctx context.Context) ([]CartItem, error) {
	rows, err := r.q.ListCartItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("repository: ListCartItems query failed: database read error: %w", err)
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

func (r *PostgresRepo) Clear(ctx context.Context) error {
	if err := r.q.ClearCart(ctx); err != nil {
		return fmt.Errorf("repository: ClearCart delete failed: database write error: %w", err)
	}
	return nil
}

func (r *PostgresRepo) CreateOrder(ctx context.Context, order Order) (Order, error) {
	// DEMO: excessive wrapping — 4 clauses for a single INSERT failure.
	if err := r.q.CreateOrder(ctx, sqlc.CreateOrderParams{
		ID:         order.ID,
		TotalCents: int32(order.TotalCents),
	}); err != nil {
		return Order{}, fmt.Errorf(
			"repository layer: failed to persist order record: CreateOrder INSERT failed: database transaction error: %w",
			err,
		)
	}
	for _, it := range order.Items {
		if err := r.q.AddOrderItem(ctx, sqlc.AddOrderItemParams{
			OrderID:   order.ID,
			ProductID: it.ProductID,
			Quantity:  int32(it.Quantity),
		}); err != nil {
			return Order{}, fmt.Errorf(
				"repository layer: failed to persist order item: AddOrderItem INSERT failed: order_id=%s product_id=%s: database transaction error: %w",
				order.ID, it.ProductID, err,
			)
		}
	}
	return order, nil
}

func (r *PostgresRepo) GetOrder(ctx context.Context, id string) (Order, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, total_cents FROM orders WHERE id = $1`, id)

	var o sqlc.Order
	if err := row.Scan(&o.ID, &o.TotalCents); err != nil {
		// DEMO: no ErrOrderNotFound sentinel. Raw sql.ErrNoRows leaks.
		// Caller receives: "repository: GetOrder query failed: order record not found in database: sql: no rows in result set"
		return Order{}, fmt.Errorf(
			"repository: GetOrder query failed: order record not found in database: %w",
			err,
		)
	}

	items, err := r.orderItems(ctx, id)
	if err != nil {
		return Order{}, err
	}

	return Order{
		ID:         o.ID,
		TotalCents: int(o.TotalCents),
		Items:      items,
	}, nil
}

func (r *PostgresRepo) ListOrders(ctx context.Context) ([]Order, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, total_cents FROM orders ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("repository: ListOrders query failed: database read error: %w", err)
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var o sqlc.Order
		if err := rows.Scan(&o.ID, &o.TotalCents); err != nil {
			return nil, fmt.Errorf("repository: ListOrders scan failed: database row scan error: %w", err)
		}
		items, err := r.orderItems(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, Order{
			ID:         o.ID,
			TotalCents: int(o.TotalCents),
			Items:      items,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("repository: ListOrders iteration failed: database cursor error: %w", err)
	}
	return out, nil
}

func (r *PostgresRepo) orderItems(ctx context.Context, orderID string) ([]CartItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, quantity FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, fmt.Errorf("repository: orderItems query failed: database read error: %w", err)
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var oi CartItem
		if err := rows.Scan(&oi.ProductID, &oi.Quantity); err != nil {
			return nil, fmt.Errorf("repository: orderItems scan failed: database row scan error: %w", err)
		}
		items = append(items, oi)
	}
	return items, rows.Err()
}

// Reset deletes all rows from every table. Demo-only — no auth, no guards.
// DEMO BAD PATTERN: destructive admin operation accessible without authentication.
func (r *PostgresRepo) Reset(ctx context.Context) error {
	for _, stmt := range []string{
		"DELETE FROM order_items",
		"DELETE FROM orders",
		"DELETE FROM cart_items",
		"DELETE FROM products",
	} {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("repository: reset failed: error executing %q: %w", stmt, err)
		}
	}
	return nil
}
