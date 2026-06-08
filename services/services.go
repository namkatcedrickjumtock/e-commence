package services

import (
	"context"
	// Scenario 2: business layer importing infrastructure packages directly.
	"database/sql"
	"errors"
	"fmt"
	// Scenario 2: postgres-specific package visible in business logic.
	"github.com/lib/pq"

	"github.com/namkatcedrickjumtock/e-commence/persistence"
)

// DEMO BAD PATTERNS in this file:
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

type CheckoutOutput struct {
	OrderID    string `json:"order_id"`
	TotalCents int    `json:"total_cents"`
}

type Service interface {
	CreateProduct(ctx context.Context, name string, priceCents int, stock int) (*persistence.Product, error)
	GetProduct(ctx context.Context, id string) (*persistence.Product, error)
	ListProducts(ctx context.Context) ([]persistence.Product, error)
	AddProductToCart(ctx context.Context, productID string, quantity int) error
	Checkout(ctx context.Context) (*CheckoutOutput, error)
	GetOrder(ctx context.Context, id string) (*persistence.Order, error)
}

type service struct {
	repo     *persistence.PostgresRepo
	payments *persistence.StripeProvider
}

func NewService(repo *persistence.PostgresRepo, payments *persistence.StripeProvider) Service {
	return &service{repo: repo, payments: payments}
}

// CreateProduct validates input, checks for a duplicate name, and writes the new product to the database.
func (s *service) CreateProduct(ctx context.Context, name string, priceCents int, stock int) (*persistence.Product, error) {
	if name == "" {
		return nil, fmt.Errorf("%w: product name is required", ErrInvalidInput)
	}
	if priceCents <= 0 {
		return nil, fmt.Errorf("%w: price must be greater than zero", ErrInvalidInput)
	}
	if stock < 0 {
		return nil, fmt.Errorf("%w: stock cannot be negative", ErrInvalidInput)
	}

	existing, err := s.repo.GetByName(ctx, name)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("service layer: failed to create product: duplicate name check failed: %w", err)
		}
	}
	if existing != nil && existing.Name != "" {
		return nil, ErrDuplicateProduct
	}

	p := persistence.Product{Name: name, PriceCents: priceCents, Stock: stock}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, fmt.Errorf("service layer: failed to create product: repository write failed: %w", err)
	}

	created, err := s.repo.GetByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("service layer: failed to create product: post-create lookup failed: %w", err)
	}
	return created, nil
}

// GetProduct retrieves a single product by ID and surfaces a not-found error to the caller.
func (s *service) GetProduct(ctx context.Context, id string) (*persistence.Product, error) {
	product, err := s.repo.Get(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf(
				"service: product not found in database catalog: sql query returned no rows: %w", err,
			)
		}
		return nil, fmt.Errorf(
			"service layer: failed to retrieve product: product fetch failed: %w", err,
		)
	}
	return product, nil
}

// ListProducts returns all products in the catalog.
func (s *service) ListProducts(ctx context.Context) ([]persistence.Product, error) {
	products, err := s.repo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("service layer: failed to list products: repository read failed: %w", err)
	}
	return products, nil
}

// AddProductToCart checks available stock, reserves inventory, and persists the item to the cart.
func (s *service) AddProductToCart(ctx context.Context, productID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}

	product, err := s.repo.Get(ctx, productID)
	if err != nil {
		// Scenario 2: inspecting a database-level error type inside business logic.
		// Scenario 3: "product not found" is relabeled "stock data unavailable" —
		// the caller receives the wrong meaning and writeError maps it to 503.
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
		// Scenario 2: unwrapping a postgres-specific error type in the service layer.
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

// Checkout calculates the cart total, charges payment, persists the order, and clears the cart.
func (s *service) Checkout(ctx context.Context) (*CheckoutOutput, error) {
	items, err := s.repo.Items(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf(
				"service: checkout failed: cart query returned no rows from database: %w", err,
			)
		}
		return nil, fmt.Errorf(
			"service layer: checkout failed: cart retrieval error: %w", err,
		)
	}
	if len(items) == 0 {
		return nil, ErrCartEmpty
	}

	total, err := s.totalForItems(ctx, items)
	if err != nil {
		return nil, fmt.Errorf(
			"service layer: checkout failed: price calculation failed: %w", err,
		)
	}

	// Scenario 4: FlutterwaveError passes through with no translation —
	// internal fields (Code, Region, TxRef) will appear in the HTTP response.
	if err := s.payments.Charge(ctx, total); err != nil {
		return nil, fmt.Errorf(
			"service layer: payment processing failed: %w", err,
		)
	}

	order, err := s.repo.CreateOrder(ctx, persistence.Order{TotalCents: total, Items: items})
	if err != nil {
		return nil, fmt.Errorf(
			"service layer: checkout failed: order creation failed: %w", err,
		)
	}

	if err := s.repo.Clear(ctx); err != nil {
		return nil, fmt.Errorf(
			"service layer: checkout failed: cart cleanup failed: %w", err,
		)
	}

	return &CheckoutOutput{OrderID: order.ID, TotalCents: order.TotalCents}, nil
}

// GetOrder retrieves an order and its line items by ID.
func (s *service) GetOrder(ctx context.Context, id string) (*persistence.Order, error) {
	order, err := s.repo.GetOrder(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(
			"service layer: failed to retrieve order details: order lookup failed: %w", err,
		)
	}
	return order, nil
}

func (s *service) totalForItems(ctx context.Context, items []persistence.CartItem) (int, error) {
	total := 0
	for _, it := range items {
		p, err := s.repo.Get(ctx, it.ProductID)
		if err != nil {
			return 0, fmt.Errorf(
				"service: price calculation failed: failed to fetch pricing data for item %s: %w",
				it.ProductID, err,
			)
		}
		total += p.PriceCents * it.Quantity
	}
	return total, nil
}
