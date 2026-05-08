package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/namkatcedrickjumtock/e-commence/persistence/sqlc"
)

// PostgresRepo wraps the sqlc-generated *Queries and exposes
// business-oriented data-access methods. Every method translates
// raw database errors into sentinel errors owned by the persistence layer
// so that callers never need to know about database/sql or PostgreSQL details.
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
		if errors.Is(err, sql.ErrNoRows) {
			return Product{}, ErrProductNotFound
		}
		return Product{}, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
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
	err := row.Scan(&p.ID, &p.Name, &p.PriceCents, &p.Stock)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Product{}, ErrProductNotFound
		}
		return Product{}, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
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
		return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	defer rows.Close()

	var out []Product
	for rows.Next() {
		var p sqlc.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.PriceCents, &p.Stock); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
		}
		out = append(out, Product{
			ID:         p.ID,
			Name:       p.Name,
			PriceCents: int(p.PriceCents),
			Stock:      int(p.Stock),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	return out, nil
}

func (r *PostgresRepo) Create(ctx context.Context, product Product) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO products (name, price_cents, stock) VALUES ($1, $2, $3)`,
		product.Name, product.PriceCents, product.Stock)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	return nil
}

func (r *PostgresRepo) Reserve(ctx context.Context, productID string, quantity int) error {
	err := r.q.ReserveStock(ctx, sqlc.ReserveStockParams{
		ID:    productID,
		Stock: int32(quantity),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	return nil
}

 

func (r *PostgresRepo) AddItem(ctx context.Context, item CartItem) error {
	err := r.q.InsertCartItem(ctx, sqlc.InsertCartItemParams{
		ProductID: item.ProductID,
		Quantity:  int32(item.Quantity),
	})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	return nil
}

func (r *PostgresRepo) Items(ctx context.Context) ([]CartItem, error) {
	rows, err := r.q.ListCartItems(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
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
		return fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	return nil
}

func (r *PostgresRepo) CreateOrder(ctx context.Context, order Order) (Order, error) {
	if err := r.q.CreateOrder(ctx, sqlc.CreateOrderParams{
		ID:         order.ID,
		TotalCents: int32(order.TotalCents),
	}); err != nil {
		return Order{}, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	for _, it := range order.Items {
		if err := r.q.AddOrderItem(ctx, sqlc.AddOrderItemParams{
			OrderID:   order.ID,
			ProductID: it.ProductID,
			Quantity:  int32(it.Quantity),
		}); err != nil {
			return Order{}, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
		}
	}
	return order, nil
}

func (r *PostgresRepo) GetOrder(ctx context.Context, id string) (Order, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, total_cents FROM orders WHERE id = $1`, id)

	var o sqlc.Order
	if err := row.Scan(&o.ID, &o.TotalCents); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Order{}, ErrOrderNotFound
		}
		return Order{}, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
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
		return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	defer rows.Close()

	var out []Order
	for rows.Next() {
		var o sqlc.Order
		if err := rows.Scan(&o.ID, &o.TotalCents); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
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
		return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	return out, nil
}

func (r *PostgresRepo) orderItems(ctx context.Context, orderID string) ([]CartItem, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT product_id, quantity FROM order_items WHERE order_id = $1`, orderID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var oi CartItem
		if err := rows.Scan(&oi.ProductID, &oi.Quantity); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrDatabaseUnavailable, err)
		}
		items = append(items, oi)
	}
	return items, rows.Err()
}
