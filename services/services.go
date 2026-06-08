package services

import (
	"context"
	"fmt"

	"github.com/namkatcedrickjumtock/e-commence/persistence"
)

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
	if err := s.repo.Create(ctx, persistence.Product{Name: name, PriceCents: priceCents, Stock: stock}); err != nil {
		return nil, err
	}
	return s.repo.GetByName(ctx, name)
}

func (s *service) GetProduct(ctx context.Context, id string) (*persistence.Product, error) {
	return s.repo.Get(ctx, id)
}

func (s *service) ListProducts(ctx context.Context) ([]persistence.Product, error) {
	return s.repo.List(ctx)
}

func (s *service) AddProductToCart(ctx context.Context, productID string, quantity int) error {
	if quantity <= 0 {
		return ErrInvalidQuantity
	}
	product, err := s.repo.Get(ctx, productID)
	if err != nil {
		return fmt.Errorf("adding to cart: %w", err)
	}
	if product.Stock < quantity {
		return persistence.ErrProductOutOfStock
	}
	if err := s.repo.Reserve(ctx, productID, quantity); err != nil {
		return err
	}
	return s.repo.AddItem(ctx, persistence.CartItem{ProductID: productID, Quantity: quantity})
}

func (s *service) Checkout(ctx context.Context) (*CheckoutOutput, error) {
	items, err := s.repo.Items(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, ErrCartEmpty
	}
	total, err := s.totalForItems(ctx, items)
	if err != nil {
		return nil, err
	}
	if err := s.payments.Charge(ctx, total); err != nil {
		return nil, err
	}
	order, err := s.repo.CreateOrder(ctx, persistence.Order{TotalCents: total, Items: items})
	if err != nil {
		return nil, err
	}
	if err := s.repo.Clear(ctx); err != nil {
		return nil, err
	}
	return &CheckoutOutput{OrderID: order.ID, TotalCents: order.TotalCents}, nil
}

func (s *service) GetOrder(ctx context.Context, id string) (*persistence.Order, error) {
	return s.repo.GetOrder(ctx, id)
}

func (s *service) totalForItems(ctx context.Context, items []persistence.CartItem) (int, error) {
	total := 0
	for _, it := range items {
		p, err := s.repo.Get(ctx, it.ProductID)
		if err != nil {
			return 0, err
		}
		total += p.PriceCents * it.Quantity
	}
	return total, nil
}
