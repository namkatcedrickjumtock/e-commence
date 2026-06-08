package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/namkatcedrickjumtock/e-commence/persistence"
	"github.com/namkatcedrickjumtock/e-commence/services"
)

type Handler struct {
	svc      services.Service
	logger   *log.Logger
	payments *persistence.StripeProvider
	repo     *persistence.PostgresRepo
}

func NewHandler(
	svc services.Service,
	logger *log.Logger,
	payments *persistence.StripeProvider,
	repo *persistence.PostgresRepo,
) *Handler {
	return &Handler{svc: svc, logger: logger, payments: payments, repo: repo}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /products", h.handleListProducts)
	mux.HandleFunc("GET /products/{id}", h.handleGetProduct)
	mux.HandleFunc("POST /cart/items", h.handleAddCartItem)
	mux.HandleFunc("POST /checkout", h.handleCheckout)
	mux.HandleFunc("GET /orders/{id}", h.handleGetOrder)
	mux.HandleFunc("POST /demo/reset", h.handleDemoReset)
	mux.HandleFunc("POST /demo/seed", h.handleDemoSeed)
	mux.HandleFunc("POST /demo/payment-mode", h.handleSetPaymentMode)
	return h.withLogging(mux)
}

// handleListProducts handles GET /products — returns all products in the catalog.
func (h *Handler) handleListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.svc.ListProducts(r.Context())
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: GET /products failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, products)
}

// handleGetProduct handles GET /products/{id} — validates the ID prefix and returns the product.
func (h *Handler) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !strings.HasPrefix(id, "p_") {
		h.writeError(w, ErrInvalidProductID)
		return
	}
	product, err := h.svc.GetProduct(r.Context(), id)
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: GET /products/{id} failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, product)
}

// handleAddCartItem handles POST /cart/items — decodes the request and delegates to svc.AddProductToCart.
func (h *Handler) handleAddCartItem(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}
	if err := readJSON(r, &req); err != nil {
		h.writeError(w, err)
		return
	}
	if strings.TrimSpace(req.ProductID) == "" {
		h.writeError(w, ErrMissingField)
		return
	}
	if !strings.HasPrefix(req.ProductID, "p_") {
		h.writeError(w, ErrInvalidProductID)
		return
	}
	if err := h.svc.AddProductToCart(r.Context(), req.ProductID, req.Quantity); err != nil {
		h.writeError(w, fmt.Errorf("handler: POST /cart/items failed: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "added"})
}

// handleCheckout handles POST /checkout — runs with a 400ms timeout and delegates to svc.Checkout.
func (h *Handler) handleCheckout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 400*time.Millisecond)
	defer cancel()
	out, err := h.svc.Checkout(ctx)
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: POST /checkout failed: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"order_id":    out.OrderID,
		"total_cents": out.TotalCents,
	})
}

// handleGetOrder handles GET /orders/{id} — validates the ID prefix and returns the order with its items.
func (h *Handler) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !strings.HasPrefix(id, "o_") {
		h.writeError(w, ErrInvalidOrderID)
		return
	}
	order, err := h.svc.GetOrder(r.Context(), id)
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: GET /orders/{id} failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, order)
}

// handleDemoReset handles POST /demo/reset — truncates all tables directly via the repository.
func (h *Handler) handleDemoReset(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Reset(r.Context()); err != nil {
		h.writeError(w, fmt.Errorf("handler: POST /demo/reset failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset"})
}

// handleDemoSeed handles POST /demo/seed — creates three GopherCon products and returns them.
func (h *Handler) handleDemoSeed(w http.ResponseWriter, r *http.Request) {
	seeds := []struct {
		name       string
		priceCents int
		stock      int
	}{
		{"GopherCon T-Shirt", 2499, 100},
		{"Go Programming Book", 3999, 50},
		{"Gopher Plush Toy", 1499, 200},
	}

	var products []*persistence.Product
	for _, s := range seeds {
		p, err := h.svc.CreateProduct(r.Context(), s.name, s.priceCents, s.stock)
		if err != nil {
			h.writeError(w, fmt.Errorf("handler: POST /demo/seed failed: %w", err))
			return
		}
		products = append(products, p)
	}
	writeJSON(w, http.StatusCreated, products)
}

// handleSetPaymentMode handles POST /demo/payment-mode — switches the Flutterwave failure mode at runtime.
func (h *Handler) handleSetPaymentMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode"`
	}
	if err := readJSON(r, &req); err != nil {
		h.writeError(w, err)
		return
	}
	h.payments.SetMode(persistence.ParsePaymentMode(req.Mode))
	writeJSON(w, http.StatusOK, map[string]any{"payment_mode": req.Mode})
}

// Scenario 1: the full error chain from every layer is dumped verbatim into the response body.
// Scenarios 3 + 4: status codes are derived by string matching — order-dependent and fragile.
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	errMsg := err.Error()
	status := http.StatusInternalServerError

	switch {
	case errors.Is(err, ErrInvalidJSON),
		errors.Is(err, ErrMissingField),
		errors.Is(err, ErrInvalidProductID),
		errors.Is(err, ErrInvalidOrderID):
		status = http.StatusBadRequest

	// Scenario 3: "unavailable" is matched before "no rows" —
	// a product-not-found reinterpreted by the service layer returns 503 instead of 404.
	case strings.Contains(errMsg, "unavailable") ||
		strings.Contains(errMsg, "timeout"):
		status = http.StatusServiceUnavailable

	// Scenario 4: payment provider codes surface here because StripeError was never translated.
	case strings.Contains(errMsg, "stripe") ||
		strings.Contains(errMsg, "card_declined"):
		status = http.StatusPaymentRequired

	case strings.Contains(errMsg, "no rows") ||
		strings.Contains(errMsg, "not found"):
		status = http.StatusNotFound

	case strings.Contains(errMsg, "out of stock") ||
		strings.Contains(errMsg, "duplicate") ||
		strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "empty"):
		status = http.StatusConflict

	case strings.Contains(errMsg, "invalid") ||
		strings.Contains(errMsg, "missing"):
		status = http.StatusBadRequest
	}

	writeJSON(w, status, map[string]any{"error": errMsg})
}

func readJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return ErrInvalidJSON
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("content-type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		h.logger.Printf("%s %s in %s", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}
