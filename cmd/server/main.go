package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"storage/internal/config"
	"storage/internal/db"
	"storage/internal/handler"
	"storage/internal/repository"
	"storage/internal/service"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("Main(0): no .env file found")
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Main(1): failed to load config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseUrl())
	if err != nil {
		log.Fatalf("Main(2): failed to connect to db: %v", err)
	}
	defer pool.Close()

	// Start Migrations
	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		log.Printf("migrations directory %s not found, trying ../migrations", migrationsDir)
		migrationsDir = "../migrations"
	}
	if err := db.RunMigrations(ctx, pool, migrationsDir); err != nil {
		log.Fatalf("Main(3): failed to run migrations: %v", err)
	}

	productRepo := repository.NewProductRepo(pool)
	productSvc := service.NewProductService(productRepo)

	uploadDir := "./uploads/products"
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("create upload dir: %v", err)
	}
	productHandler := handler.NewProductHundler(productSvc, uploadDir)

	mux := http.NewServeMux()
	productHandler.RegisterRoutes(mux)

	//Health-check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("Main(4): starting server on port %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Main(5): listen error: %v", err)
		}
	}()

	<-quit
	log.Printf("Main(6): shutting down server...")

	shutdownCtx, cancelShut := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShut()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Main(7): server forced to shutdown: %v", err)
	}

	log.Println("server stopped")
}
