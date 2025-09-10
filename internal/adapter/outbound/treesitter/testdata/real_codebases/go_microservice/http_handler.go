package microservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// HTTPHandler handles HTTP requests for the user service
type HTTPHandler struct {
	service *UserService
	logger  *zap.Logger
}

// NewHTTPHandler creates a new HTTP handler instance
func NewHTTPHandler(service *UserService, logger *zap.Logger) *HTTPHandler {
	return &HTTPHandler{
		service: service,
		logger:  logger,
	}
}

// RegisterRoutes registers all HTTP routes for the handler
func (h *HTTPHandler) RegisterRoutes(router *mux.Router) {
	api := router.PathPrefix("/api/v1").Subrouter()
	
	// User endpoints
	api.HandleFunc("/users", h.CreateUser).Methods("POST")
	api.HandleFunc("/users", h.ListUsers).Methods("GET")
	api.HandleFunc("/users/{id}", h.GetUser).Methods("GET")
	api.HandleFunc("/users/{id}", h.UpdateUser).Methods("PUT")
	api.HandleFunc("/users/{id}", h.DeleteUser).Methods("DELETE")
	
	// Health check
	api.HandleFunc("/health", h.HealthCheck).Methods("GET")
}

// CreateUser handles POST /api/v1/users
func (h *HTTPHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode request body", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.service.CreateUser(ctx, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusCreated, user)
}

// GetUser handles GET /api/v1/users/{id}
func (h *HTTPHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	idStr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	user, err := h.service.GetUserByID(ctx, id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, user)
}

// UpdateUser handles PUT /api/v1/users/{id}
func (h *HTTPHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	idStr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	var req UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("failed to decode request body", zap.Error(err))
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := h.service.UpdateUser(ctx, id, &req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, user)
}

// DeleteUser handles DELETE /api/v1/users/{id}
func (h *HTTPHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	idStr := mux.Vars(r)["id"]
	id, err := uuid.Parse(idStr)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid user ID")
		return
	}

	if err := h.service.DeleteUser(ctx, id); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListUsers handles GET /api/v1/users
func (h *HTTPHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	// Parse query parameters
	limit, err := h.parseIntParam(r, "limit", 10)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid limit parameter")
		return
	}

	offset, err := h.parseIntParam(r, "offset", 0)
	if err != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid offset parameter")
		return
	}

	req := &ListUsersRequest{
		Limit:  limit,
		Offset: offset,
	}

	response, err := h.service.ListUsers(ctx, req)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.writeJSONResponse(w, http.StatusOK, response)
}

// HealthCheck handles GET /api/v1/health
func (h *HTTPHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "user-service",
		"timestamp": "2023-01-01T00:00:00Z",
	}
	
	h.writeJSONResponse(w, http.StatusOK, response)
}

// Helper methods

// parseIntParam parses an integer parameter from the request query string
func (h *HTTPHandler) parseIntParam(r *http.Request, param string, defaultValue int) (int, error) {
	valueStr := r.URL.Query().Get(param)
	if valueStr == "" {
		return defaultValue, nil
	}
	
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return 0, fmt.Errorf("invalid %s parameter: %w", param, err)
	}
	
	return value, nil
}

// handleServiceError handles service errors and maps them to HTTP responses
func (h *HTTPHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUserNotFound):
		h.writeErrorResponse(w, http.StatusNotFound, "user not found")
	case errors.Is(err, ErrInvalidUserData):
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid user data")
	case errors.Is(err, ErrDuplicateEmail):
		h.writeErrorResponse(w, http.StatusConflict, "email already exists")
	default:
		h.logger.Error("internal server error", zap.Error(err))
		h.writeErrorResponse(w, http.StatusInternalServerError, "internal server error")
	}
}

// writeJSONResponse writes a JSON response with the given status code and data
func (h *HTTPHandler) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode JSON response", zap.Error(err))
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

// writeErrorResponse writes an error response with the given status code and message
func (h *HTTPHandler) writeErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	errorResponse := ErrorResponse{
		Error: ErrorDetails{
			Code:    statusCode,
			Message: message,
		},
	}
	
	h.writeJSONResponse(w, statusCode, errorResponse)
}

// CORS middleware
func (h *HTTPHandler) EnableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Logging middleware
func (h *HTTPHandler) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		
		next.ServeHTTP(wrapped, r)
		
		duration := time.Since(start)
		
		h.logger.Info("HTTP request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("user_agent", r.UserAgent()),
			zap.Int("status", wrapped.statusCode),
			zap.Duration("duration", duration),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Error response types

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error ErrorDetails `json:"error"`
}

// ErrorDetails represents error details
type ErrorDetails struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// AuthenticationMiddleware provides JWT authentication
func (h *HTTPHandler) AuthenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health check
		if strings.HasSuffix(r.URL.Path, "/health") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			h.writeErrorResponse(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		// Simple token validation (in real implementation, validate JWT)
		if !strings.HasPrefix(authHeader, "Bearer ") {
			h.writeErrorResponse(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			h.writeErrorResponse(w, http.StatusUnauthorized, "missing token")
			return
		}

		// In real implementation, validate the JWT token here
		// For this example, we'll accept any non-empty token
		
		next.ServeHTTP(w, r)
	})
}

import (
	"errors"
	"time"
)