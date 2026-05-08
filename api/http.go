// Package api is the presentation layer.
// It owns HTTP-specific concerns: request parsing, response serialization,
// and mapping every possible error to the correct HTTP status code.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/namkatcedrickjumtock/e-commence/persistence"
	"github.com/namkatcedrickjumtock/e-commence/services"
)

type Handler struct {
	svc    services.Service
	logger *log.Logger
}

func NewHandler(svc services.Service, logger *log.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
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
	return h.withLogging(mux)
}

func (h *Handler) handleListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.svc.ListProducts(r.Context())
	if err != nil {
		h.writeError(w, err)
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
		h.writeError(w, err)
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
		h.writeError(w, err)
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
		h.writeError(w, err)
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
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"status": "added"})
}

func (h *Handler) handleCheckout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 400*time.Millisecond)
	defer cancel()
	out, err := h.svc.Checkout(ctx)
	if err != nil {
		h.writeError(w, err)
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
		h.writeError(w, err)
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
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, order)
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

// writeError maps every known sentinel error to an HTTP status.
// Errors are checked by layer: presentation → business → persistence.
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError

	switch {
	// Presentation-layer errors (api/)
	case errors.Is(err, ErrInvalidJSON),
		errors.Is(err, ErrMissingField),
		errors.Is(err, ErrInvalidProductID),
		errors.Is(err, ErrInvalidOrderID),
		errors.Is(err, ErrInvalidName),
		errors.Is(err, ErrInvalidPrice),
		errors.Is(err, ErrInvalidStock):
		status = http.StatusBadRequest

	// Business-layer errors (services/)
	case errors.Is(err, services.ErrDuplicateProduct):
		status = http.StatusConflict
	case errors.Is(err, services.ErrInvalidQuantity),
		errors.Is(err, services.ErrInvalidInput):
		status = http.StatusBadRequest
	case errors.Is(err, services.ErrCartEmpty):
		status = http.StatusConflict
	case errors.Is(err, services.ErrPaymentDeclined):
		status = http.StatusPaymentRequired

	// Persistence-layer errors (persistence/)
	case errors.Is(err, persistence.ErrProductNotFound),
		errors.Is(err, persistence.ErrOrderNotFound):
		status = http.StatusNotFound
	case errors.Is(err, persistence.ErrProductOutOfStock):
		status = http.StatusConflict
	case errors.Is(err, persistence.ErrDuplicateCartItem):
		status = http.StatusConflict
	case errors.Is(err, persistence.ErrStripeCardDeclined):
		status = http.StatusPaymentRequired
	case errors.Is(err, persistence.ErrStripeProcessingError):
		status = http.StatusUnprocessableEntity
	case errors.Is(err, persistence.ErrDatabaseUnavailable),
		errors.Is(err, persistence.ErrPaymentProviderTimeout),
		errors.Is(err, persistence.ErrStripeRateLimit):
		status = http.StatusServiceUnavailable
	case errors.Is(err, persistence.ErrStripeAPIError):
		status = http.StatusBadGateway
	}

	writeJSON(w, status, map[string]any{"error": err.Error()})
}

func (h *Handler) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		h.logger.Printf("%s %s in %s", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}
