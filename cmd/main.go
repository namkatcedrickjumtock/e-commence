package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"e-commence/internal/api"
	"e-commence/internal/application"
	"e-commence/internal/infrastructure"
)

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	store := infrastructure.NewInMemoryStore()

	paymentMode := os.Getenv("DEMO_PAYMENT_MODE") // ok | decline | timeout
	payments := infrastructure.NewFakePaymentProvider(paymentMode)

	useCases := application.NewUseCases(store, store, store, payments)
	handler := api.NewHandler(useCases, logger)

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           handler.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Printf("listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("server error: %v", err)
	}
}

