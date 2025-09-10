package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"./handlers"
	"./services"
)

func main() {
	// Initialize services
	userService := services.NewUserService()

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler()
	userHandler := handlers.NewUserHandler(userService)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler.Health)
	mux.HandleFunc("/api/users", userHandler.HandleUsers)
	mux.HandleFunc("/api/users/", userHandler.HandleUserByID)

	// Create server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start server in goroutine
	go func() {
		log.Println("Starting server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}