package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"e-commence/internal/application"
	"e-commence/internal/domain"
	"e-commence/internal/infrastructure"
)

type Handler struct {
	uc     *application.UseCases
	logger *log.Logger
}

func NewHandler(uc *application.UseCases, logger *log.Logger) *Handler {
	return &Handler{uc: uc, logger: logger}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /cart/items", h.handleAddCartItem)
	mux.HandleFunc("POST /checkout", h.handleCheckout)

	return h.withLogging(mux)
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

	if err := h.uc.AddProductToCart(r.Context(), application.AddToCartInput{
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
	}); err != nil {
		h.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"status": "added"})
}

func (h *Handler) handleCheckout(w http.ResponseWriter, r *http.Request) {
	// Small timeout so the "payment timeout" demo is easy.
	ctx, cancel := context.WithTimeout(r.Context(), 400*time.Millisecond)
	defer cancel()

	out, err := h.uc.Checkout(ctx)
	if err != nil {
		h.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"order_id":    out.OrderID,
		"total_cents": out.TotalCents,
	})
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

func (h *Handler) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError

	switch {
	// Presentation errors
	case errors.Is(err, ErrInvalidJSON),
		errors.Is(err, ErrMissingField),
		errors.Is(err, ErrInvalidProductID):
		status = http.StatusBadRequest

	// Domain errors
	case errors.Is(err, domain.ErrProductNotFound):
		status = http.StatusNotFound
	case errors.Is(err, domain.ErrDuplicateCartItem):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrProductOutOfStock):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrInvalidQuantity):
		status = http.StatusBadRequest
	case errors.Is(err, domain.ErrCartEmpty):
		status = http.StatusConflict
	case errors.Is(err, domain.ErrPaymentDeclined):
		status = http.StatusPaymentRequired

	// Infrastructure errors
	case errors.Is(err, infrastructure.ErrDatabaseUnavailable),
		errors.Is(err, infrastructure.ErrPaymentProviderTimeout):
		status = http.StatusServiceUnavailable
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

