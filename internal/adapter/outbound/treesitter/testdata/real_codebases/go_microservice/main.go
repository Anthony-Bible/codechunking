package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	ctx := context.Background()
	
	server := &http.Server{
		Addr:    ":8080",
		Handler: setupRoutes(),
	}

	go func() {
		log.Println("Server starting on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	gracefulShutdown(ctx, server)
}

func setupRoutes() http.Handler {
	mux := http.NewServeMux()
	
	mux.HandleFunc("/health", healthCheckHandler)
	mux.HandleFunc("/api/users", userHandler)
	mux.HandleFunc("/api/data", dataHandler)
	
	return mux
}

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"status": "healthy", "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
}

func userHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		getUsersHandler(w, r)
	case http.MethodPost:
		createUserHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getUsersHandler(w http.ResponseWriter, r *http.Request) {
	users := []map[string]interface{}{
		{"id": 1, "name": "John Doe", "email": "john@example.com"},
		{"id": 2, "name": "Jane Smith", "email": "jane@example.com"},
	}
	
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"users": %v}`, users)
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	// Simplified user creation
	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, `{"message": "User created successfully"}`)
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	data := processData([]string{"item1", "item2", "item3"})
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"data": %v}`, data)
}

func processData(items []string) []map[string]interface{} {
	var results []map[string]interface{}
	
	for i, item := range items {
		result := map[string]interface{}{
			"id":        i + 1,
			"original":  item,
			"processed": fmt.Sprintf("processed_%s", item),
			"timestamp": time.Now().Unix(),
		}
		results = append(results, result)
	}
	
	return results
}

func gracefulShutdown(ctx context.Context, server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	<-quit
	log.Println("Shutting down server...")
	
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}
	
	log.Println("Server exited")
}