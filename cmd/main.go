package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ardanlabs/conf/v3"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/namkatcedrickjumtock/e-commence/api"
	"github.com/namkatcedrickjumtock/e-commence/persistence"
	"github.com/namkatcedrickjumtock/e-commence/services"
)

type config struct {
	Web struct {
		Addr              string        `conf:"env:ADDR,default::8080"`
		ReadHeaderTimeout time.Duration `conf:"env:READ_HEADER_TIMEOUT,default:5s"`
	}
	DB struct {
		User     string `conf:"env:DB_USER,default:adminuser"`
		Password string `conf:"env:DB_PASSWORD,default:postgres"`
		Host     string `conf:"env:DB_HOST,default:localhost"`
		Port     int    `conf:"env:DB_PORT,default:5432"`
		Name     string `conf:"env:DB_NAME,default:postgres"`
	}
	Payment struct {
		Mode string `conf:"env:PAYMENT_MODE,default:ok"`
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	if _, err := os.Stat(".env"); err == nil {
		if err := godotenv.Load(); err != nil {
			return fmt.Errorf("loading .env: %w", err)
		}
	}

	var cfg config
	help, err := conf.Parse("", &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	logger := log.New(os.Stdout, "", log.LstdFlags)

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.DB.Host, cfg.DB.Port, cfg.DB.User, cfg.DB.Password, cfg.DB.Name,
	)
	sqlDB, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.PingContext(context.Background()); err != nil {
		return fmt.Errorf("connecting to database: %w", err)
	}
	logger.Println("connected to postgres")

	repo := persistence.NewPostgresRepo(sqlDB)
	payments := persistence.NewFlutterwaveProvider(persistence.ParsePaymentMode(cfg.Payment.Mode))
	svc := services.NewService(repo, payments)

	// DEMO BAD PATTERN: handler receives direct references to infrastructure
	// (payments + repo) so demo endpoints can bypass the service layer.
	handler := api.NewHandler(svc, logger, payments, repo)

	srv := &http.Server{
		Addr:              cfg.Web.Addr,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: cfg.Web.ReadHeaderTimeout,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		logger.Printf("listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("server error: %v", err)
		}
	}()

	<-shutdown
	logger.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return srv.Shutdown(ctx)
}
