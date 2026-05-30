// Package api is the presentation layer.
//
// DEMO BAD PATTERNS in this file:
//
//  1. writeError uses strings.Contains instead of errors.Is — fragile, order-dependent,
//     and easily broken by any change to an error message anywhere in the stack.
//
//  2. The Handler struct holds a direct reference to *persistence.FlutterwaveProvider
//     and *persistence.PostgresRepo — the HTTP layer is coupled to infrastructure.
//
//  3. Every handler wraps its error before passing to writeError, adding yet another
//     "handler: ..." prefix to an already-verbose chain.
//
//  4. The /demo/* routes access the repository directly, bypassing the service layer.
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

// Handler holds more than it should.
// DEMO BAD PATTERN: direct infrastructure references in the HTTP layer.
type Handler struct {
	svc      services.Service
	logger   *log.Logger
	payments *persistence.FlutterwaveProvider // bypasses service abstraction
	repo     *persistence.PostgresRepo        // bypasses service abstraction
}

func NewHandler(
	svc services.Service,
	logger *log.Logger,
	payments *persistence.FlutterwaveProvider,
	repo *persistence.PostgresRepo,
) *Handler {
	return &Handler{svc: svc, logger: logger, payments: payments, repo: repo}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /products", h.handleListProducts)
	mux.HandleFunc("POST /products", h.handleCreateProduct)
	mux.HandleFunc("GET /products/{id}", h.handleGetProduct)
	mux.HandleFunc("GET /products/{id}/inventory", h.handleCheckInventory)
	mux.HandleFunc("POST /cart/items", h.handleAddCartItem)
	mux.HandleFunc("POST /checkout", h.handleCheckout)
	mux.HandleFunc("GET /orders", h.handleListOrders)
	mux.HandleFunc("GET /orders/{id}", h.handleGetOrder)

	// Demo-only management endpoints.
	// DEMO BAD PATTERN: destructive/admin operations reachable over HTTP with no auth.
	mux.HandleFunc("POST /demo/reset", h.handleDemoReset)
	mux.HandleFunc("POST /demo/seed", h.handleDemoSeed)
	mux.HandleFunc("POST /demo/payment-mode", h.handleSetPaymentMode)

	return h.withLogging(mux)
}

// ── product handlers ──────────────────────────────────────────────────────────

func (h *Handler) handleListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.svc.ListProducts(r.Context())
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: GET /products failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, products)
}

func (h *Handler) handleCreateProduct(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string `json:"name"`
		PriceCents int    `json:"price_cents"`
		Stock      int    `json:"stock"`
	}
	if err := readJSON(r, &req); err != nil {
		h.writeError(w, err)
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		h.writeError(w, ErrInvalidName)
		return
	}
	if req.PriceCents <= 0 {
		h.writeError(w, ErrInvalidPrice)
		return
	}
	if req.Stock < 0 {
		h.writeError(w, ErrInvalidStock)
		return
	}
	product, err := h.svc.CreateProduct(r.Context(), req.Name, req.PriceCents, req.Stock)
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: POST /products failed: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

func (h *Handler) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !strings.HasPrefix(id, "p_") {
		h.writeError(w, ErrInvalidProductID)
		return
	}
	product, err := h.svc.GetProduct(r.Context(), id)
	if err != nil {
		// DEMO SCENARIO 1 + 2: handler wraps, adding to the chain.
		// The full message will contain "sql: no rows in result set" from the persistence
		// layer, even though this is an HTTP handler that should know nothing about SQL.
		h.writeError(w, fmt.Errorf("handler: GET /products/{id} failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, product)
}

func (h *Handler) handleCheckInventory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !strings.HasPrefix(id, "p_") {
		h.writeError(w, ErrInvalidProductID)
		return
	}
	stock, err := h.svc.CheckInventory(r.Context(), id)
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: GET /products/{id}/inventory failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"product_id": id,
		"stock":      stock,
	})
}

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
		// DEMO SCENARIO 3: the handler wraps an error that already has its meaning
		// reinterpreted by the service layer. "product not found" became
		// "stock data unavailable" in services.go, and writeError will map
		// "unavailable" → 503 instead of the correct 404.
		h.writeError(w, fmt.Errorf("handler: POST /cart/items failed: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "added"})
}

func (h *Handler) handleCheckout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 400*time.Millisecond)
	defer cancel()
	out, err := h.svc.Checkout(ctx)
	if err != nil {
		// DEMO SCENARIO 1 + 4: handler adds its own wrap.
		// When payment fails, the final message will contain the raw
		// FlutterwaveError text: "FW-9082 region=eu-west retryable=false ..."
		h.writeError(w, fmt.Errorf("handler: POST /checkout failed: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"order_id":    out.OrderID,
		"total_cents": out.TotalCents,
	})
}

func (h *Handler) handleListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.svc.ListOrders(r.Context())
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: GET /orders failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (h *Handler) handleGetOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !strings.HasPrefix(id, "o_") {
		h.writeError(w, ErrInvalidOrderID)
		return
	}
	order, err := h.svc.GetOrder(r.Context(), id)
	if err != nil {
		// DEMO SCENARIO 1: the full chain becomes:
		// "handler: GET /orders/{id} failed:
		//   service layer: failed to retrieve order details: order lookup failed:
		//   repository: GetOrder query failed: order record not found in database:
		//   sql: no rows in result set"
		h.writeError(w, fmt.Errorf("handler: GET /orders/{id} failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, order)
}

// ── demo management endpoints ─────────────────────────────────────────────────

// handleDemoReset clears every table.
// DEMO BAD PATTERN: HTTP handler calls repository directly, bypassing service layer.
func (h *Handler) handleDemoReset(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Reset(r.Context()); err != nil {
		h.writeError(w, fmt.Errorf("handler: POST /demo/reset failed: %w", err))
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset"})
}

// handleDemoSeed inserts a canonical demo product.
func (h *Handler) handleDemoSeed(w http.ResponseWriter, r *http.Request) {
	product, err := h.svc.CreateProduct(r.Context(), "Demo Widget", 2999, 50)
	if err != nil {
		h.writeError(w, fmt.Errorf("handler: POST /demo/seed failed: %w", err))
		return
	}
	writeJSON(w, http.StatusCreated, product)
}

// handleSetPaymentMode changes the Flutterwave provider's failure mode at runtime.
// DEMO BAD PATTERN: HTTP handler mutates infrastructure state directly.
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

// ── error mapping ─────────────────────────────────────────────────────────────

// writeError maps errors to HTTP status codes using strings.Contains.
//
// DEMO BAD PATTERNS:
//   - No errors.Is — the entire error contract is encoded in magic strings.
//     Renaming any error message anywhere in the codebase silently breaks routing.
//   - Case ordering matters and is load-bearing: "unavailable" is checked before
//     "no rows", so a product-not-found error reinterpreted as "stock data unavailable"
//     by the service layer produces 503 instead of 404 (Scenario 3).
//   - The full error chain — including SQL internals and provider codes — is dumped
//     verbatim into the response body.
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	errMsg := err.Error()
	status := http.StatusInternalServerError

	switch {
	// Input validation — matched by magic strings.
	case errors.Is(err, ErrInvalidJSON),
		errors.Is(err, ErrMissingField),
		errors.Is(err, ErrInvalidProductID),
		errors.Is(err, ErrInvalidOrderID),
		errors.Is(err, ErrInvalidName),
		errors.Is(err, ErrInvalidPrice),
		errors.Is(err, ErrInvalidStock):
		status = http.StatusBadRequest

	// String matching from here down — order is load-bearing.

	// "unavailable" before "no rows": a product-not-found reinterpreted as
	// "stock data unavailable" by the service will hit this branch → 503.
	case strings.Contains(errMsg, "unavailable") ||
		strings.Contains(errMsg, "timeout"):
		status = http.StatusServiceUnavailable

	// Payment provider leakage: FW- codes and "declined" reach here verbatim.
	case strings.Contains(errMsg, "FW-") ||
		strings.Contains(errMsg, "declined"):
		status = http.StatusPaymentRequired

	// These only fire if the service didn't reinterpret "not found" first.
	case strings.Contains(errMsg, "no rows") ||
		strings.Contains(errMsg, "not found"):
		status = http.StatusNotFound

	case strings.Contains(errMsg, "out of stock") ||
		strings.Contains(errMsg, "constraint") ||
		strings.Contains(errMsg, "duplicate") ||
		strings.Contains(errMsg, "already exists") ||
		strings.Contains(errMsg, "empty"):
		status = http.StatusConflict

	case strings.Contains(errMsg, "invalid") ||
		strings.Contains(errMsg, "missing") ||
		strings.Contains(errMsg, "invalid quantity"):
		status = http.StatusBadRequest
	}

	writeJSON(w, status, map[string]any{"error": errMsg})
}

// ── helpers ───────────────────────────────────────────────────────────────────

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
