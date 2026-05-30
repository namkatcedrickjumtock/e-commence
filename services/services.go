// Package services is the business layer.
//
// DEMO BAD PATTERNS in this file:
//
//  1. Imports "database/sql" and "github.com/lib/pq" — the business layer is
//     directly coupled to persistence infrastructure.
//
//  2. Calls errors.Is(err, sql.ErrNoRows) and errors.As(err, &pgErr) throughout —
//     business logic branches on database-level error types it should never see.
//
//  3. Semantic reinterpretation: AddProductToCart receives "product not found"
//     (sql.ErrNoRows) and re-labels it "inventory check failed: stock data
//     unavailable", changing the meaning and causing the wrong HTTP status.
//
//  4. Every error path adds another fmt.Errorf wrap, duplicating context
//     already present in the underlying message.
package services

import (
	"context"
	"database/sql" // DEMO SCENARIO 2: business layer imports database/sql
	"errors"
	"fmt"

	// DEMO SCENARIO 2: business layer imports postgres-specific package
	"github.com/lib/pq"

	"github.com/namkatcedrickjumtock/e-commence/persistence"
)

type CheckoutOutput struct {
	OrderID    string `json:"order_id"`
	TotalCents int    `json:"total_cents"`
}

type Service interface {
	CreateProduct(ctx context.Context, name string, priceCents int, stock int) (persistence.Product, error)
	GetProduct(ctx context.Context, id string) (persistence.Product, error)
	ListProducts(ctx context.Context) ([]persistence.Product, error)
	CheckInventory(ctx context.Context, productID string) (int, error)
	AddProductToCart(ctx context.Context, productID string, quantity int) error
	Checkout(ctx context.Context) (CheckoutOutput, error)
	GetOrder(ctx context.Context, id string) (persistence.Order, error)
	ListOrders(ctx context.Context) ([]persistence.Order, error)
}

type service struct {
	repo     *persistence.PostgresRepo
	payments *persistence.FlutterwaveProvider
}

func NewService(repo *persistence.PostgresRepo, payments *persistence.FlutterwaveProvider) Service {
	return &service{repo: repo, payments: payments}
}

func (s *service) CreateProduct(ctx context.Context, name string, priceCents int, stock int) (persistence.Product, error) {
	if name == "" {
		return persistence.Product{}, fmt.Errorf("%w: product name is required", ErrInvalidInput)
	}
	if priceCents <= 0 {
		return persistence.Product{}, fmt.Errorf("%w: price must be greater than zero", ErrInvalidInput)
	}
	if stock < 0 {
		return persistence.Product{}, fmt.Errorf("%w: stock cannot be negative", ErrInvalidInput)
	}

	existing, err := s.repo.GetByName(ctx, name)
	if err != nil {
		// DEMO SCENARIO 2: business layer checks sql.ErrNoRows directly.
		// "not found" and "error" are handled by inspecting the database error type.
		if !errors.Is(err, sql.ErrNoRows) {
			return persistence.Product{}, fmt.Errorf("service layer: failed to create product: duplicate name check failed: %w", err)
		}
		// sql.ErrNoRows means the product doesn't exist yet — proceed.
	}
	if existing.Name != "" {
		return persistence.Product{}, ErrDuplicateProduct
	}

	p := persistence.Product{Name: name, PriceCents: priceCents, Stock: stock}
	if err := s.repo.Create(ctx, p); err != nil {
		return persistence.Product{}, fmt.Errorf("service layer: failed to create product: repository write failed: %w", err)
	}

	created, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return persistence.Product{}, fmt.Errorf("service layer: failed to create product: post-create lookup failed: %w", err)
	}
	return created, nil
}

func (s *service) GetProduct(ctx context.Context, id string) (persistence.Product, error) {
	product, err := s.repo.Get(ctx, id)
	if err != nil {
		// DEMO SCENARIO 2: SQL check in the business layer.
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.Product{}, fmt.Errorf(
				"service: product not found in database catalog: sql query returned no rows: %w", err,
			)
		}
		return persistence.Product{}, fmt.Errorf(
			"service layer: failed to retrieve product: product fetch failed: %w", err,
		)
	}
	return product, nil
}

func (s *service) ListProducts(ctx context.Context) ([]persistence.Product, error) {
	products, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("service layer: failed to list products: repository read failed: %w", err)
	}
	return products, nil
}

func (s *service) CheckInventory(ctx context.Context, productID string) (int, error) {
	product, err := s.repo.Get(ctx, productID)
	if err != nil {
		// DEMO SCENARIO 2: checking sql error in business layer.
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("service: inventory check failed: product not in database: sql: no rows: %w", err)
		}
		return 0, fmt.Errorf("service layer: inventory check failed: product lookup failed: %w", err)
	}
	return product.Stock, nil
}

func (s *service) AddProductToCart(ctx context.Context, productID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	product, err := s.repo.Get(ctx, productID)
	if err != nil {
		// DEMO SCENARIO 2 + 3: business layer checks sql.ErrNoRows (leakage),
		// then relabels it as "inventory check failed: stock data unavailable" (reinterpretation).
		//
		// The original meaning — "this product ID does not exist" — is lost.
		// The HTTP layer will receive "stock data unavailable" and map it to 503
		// instead of the correct 404.
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf(
				"service: cart operation failed: inventory check failed: stock data unavailable: %w",
				err,
			)
		}
		return fmt.Errorf("service: cart operation failed: failed to retrieve product data: %w", err)
	}

	if product.Stock < quantity {
		return persistence.ErrProductOutOfStock
	}

	if err := s.repo.Reserve(ctx, productID, quantity); err != nil {
		return fmt.Errorf("service: cart operation failed: inventory reservation failed: %w", err)
	}

	if err := s.repo.AddItem(ctx, persistence.CartItem{ProductID: productID, Quantity: quantity}); err != nil {
		// DEMO SCENARIO 2: business layer inspects postgres-specific error type.
		// pq.Error is an infrastructure detail that should never be visible here.
		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf(
				"service: cart operation failed: database rejected duplicate cart entry: postgres unique violation on cart_items_pkey: %w",
				err,
			)
		}
		return fmt.Errorf("service: cart operation failed: failed to persist cart item: %w", err)
	}
	return nil
}

func (s *service) Checkout(ctx context.Context) (CheckoutOutput, error) {
	items, err := s.repo.Items(ctx)
	if err != nil {
		// DEMO SCENARIO 2: business layer checks sql.ErrNoRows.
		if errors.Is(err, sql.ErrNoRows) {
			return CheckoutOutput{}, fmt.Errorf(
				"service: checkout failed: cart query returned no rows from database: %w", err,
			)
		}
		return CheckoutOutput{}, fmt.Errorf(
			"service layer: checkout failed: cart retrieval error: %w", err,
		)
	}
	if len(items) == 0 {
		return CheckoutOutput{}, ErrCartEmpty
	}

	total, err := s.totalForItems(ctx, items)
	if err != nil {
		return CheckoutOutput{}, fmt.Errorf(
			"service layer: checkout failed: price calculation failed: %w", err,
		)
	}

	if err := s.payments.Charge(ctx, total); err != nil {
		// DEMO SCENARIO 4: no payment error abstraction.
		// FlutterwaveError (with internal codes, region, tx_ref) passes through unwrapped.
		// The raw provider internals will reach the HTTP response body.
		return CheckoutOutput{}, fmt.Errorf(
			"service layer: payment processing failed: %w", err,
		)
	}

	order, err := s.repo.CreateOrder(ctx, persistence.Order{TotalCents: total, Items: items})
	if err != nil {
		return CheckoutOutput{}, fmt.Errorf(
			"service layer: checkout failed: order creation failed: %w", err,
		)
	}

	if err := s.repo.Clear(ctx); err != nil {
		return CheckoutOutput{}, fmt.Errorf(
			"service layer: checkout failed: cart cleanup failed: %w", err,
		)
	}

	return CheckoutOutput{OrderID: order.ID, TotalCents: order.TotalCents}, nil
}

func (s *service) GetOrder(ctx context.Context, id string) (persistence.Order, error) {
	// DEMO SCENARIO 1: every layer adds its own wrapper.
	// The chain will read: "service layer: failed to retrieve order details:
	//   order lookup failed: repository: GetOrder query failed:
	//   order record not found in database: sql: no rows in result set"
	order, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return persistence.Order{}, fmt.Errorf(
			"service layer: failed to retrieve order details: order lookup failed: %w", err,
		)
	}
	return order, nil
}

func (s *service) ListOrders(ctx context.Context) ([]persistence.Order, error) {
	orders, err := s.repo.ListOrders(ctx)
	if err != nil {
		return nil, fmt.Errorf("service layer: failed to list orders: repository read failed: %w", err)
	}
	return orders, nil
}

func (s *service) totalForItems(ctx context.Context, items []persistence.CartItem) (int, error) {
	total := 0
	for _, it := range items {
		p, err := s.repo.Get(ctx, it.ProductID)
		if err != nil {
			// DEMO SCENARIO 3: "product not found" reinterpreted as "price lookup failed"
			// during totalForItems — the original cause (missing product) is buried under
			// pricing context that makes no sense to the caller.
			return 0, fmt.Errorf(
				"service: price calculation failed: failed to fetch pricing data for item %s: %w",
				it.ProductID, err,
			)
		}
		total += p.PriceCents * it.Quantity
	}
	return total, nil
}
