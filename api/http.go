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

func (h *Handler) handleListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.svc.ListProducts(r.Context())
	if err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, products)
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

func (h *Handler) handleDemoReset(w http.ResponseWriter, r *http.Request) {
	if err := h.repo.Reset(r.Context()); err != nil {
		h.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "reset"})
}

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
			h.writeError(w, err)
			return
		}
		products = append(products, p)
	}
	writeJSON(w, http.StatusCreated, products)
}

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

// writeError maps domain sentinel errors to HTTP status codes via errors.Is.
// No string matching — order-independent and exhaustive.
func (h *Handler) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError

	switch {
	case errors.Is(err, ErrInvalidJSON),
		errors.Is(err, ErrMissingField),
		errors.Is(err, ErrInvalidProductID),
		errors.Is(err, ErrInvalidOrderID):
		status = http.StatusBadRequest

	case errors.Is(err, persistence.ErrProductNotFound),
		errors.Is(err, persistence.ErrOrderNotFound):
		status = http.StatusNotFound

	case errors.Is(err, persistence.ErrProductOutOfStock):
		status = http.StatusConflict

	default:
		var payErr *persistence.PaymentError
		if errors.As(err, &payErr) {
			if payErr.Retryable {
				status = http.StatusServiceUnavailable
			} else {
				status = http.StatusPaymentRequired
			}
		}
	}

	writeJSON(w, status, map[string]any{"error": err.Error()})
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
