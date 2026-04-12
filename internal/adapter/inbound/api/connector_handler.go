package api

import (
	"codechunking/internal/application/common"
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/port/inbound"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// ConnectorHandler handles HTTP requests for connector operations.
type ConnectorHandler struct {
	connectorService inbound.ConnectorService
	errorHandler     ErrorHandler
}

// NewConnectorHandler creates a new ConnectorHandler.
func NewConnectorHandler(connectorService inbound.ConnectorService, errorHandler ErrorHandler) *ConnectorHandler {
	return &ConnectorHandler{
		connectorService: connectorService,
		errorHandler:     errorHandler,
	}
}

// CreateConnector handles POST /connectors.
func (h *ConnectorHandler) CreateConnector(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateConnectorRequest
	if err := h.decodeJSON(r, &req); err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	if err := req.Validate(); err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.connectorService.CreateConnector(r.Context(), req)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	slogger.Info(r.Context(), "Connector created", slogger.Fields{"id": response.ID})
	h.writeJSON(w, http.StatusAccepted, response)
}

// ListConnectors handles GET /connectors.
func (h *ConnectorHandler) ListConnectors(w http.ResponseWriter, r *http.Request) {
	query := dto.DefaultConnectorListQuery()

	response, err := h.connectorService.ListConnectors(r.Context(), query)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetConnector handles GET /connectors/{id}.
func (h *ConnectorHandler) GetConnector(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractConnectorID(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.connectorService.GetConnector(r.Context(), id)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, http.StatusOK, response)
}

// DeleteConnector handles DELETE /connectors/{id}.
func (h *ConnectorHandler) DeleteConnector(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractConnectorID(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	if err := h.connectorService.DeleteConnector(r.Context(), id); err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SyncConnector handles POST /connectors/{id}/sync.
func (h *ConnectorHandler) SyncConnector(w http.ResponseWriter, r *http.Request) {
	id, err := h.extractConnectorID(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.connectorService.SyncConnector(r.Context(), id)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	h.writeJSON(w, http.StatusAccepted, response)
}

// extractConnectorID extracts and validates the "id" path parameter as a UUID.
func (h *ConnectorHandler) extractConnectorID(r *http.Request) (uuid.UUID, error) {
	idStr := r.PathValue("id")
	if idStr == "" {
		// Fall back to testutil path params for tests.
		idStr = r.Header.Get("X-Path-Param-id")
	}
	// Try path params map stored in context (set by testutil.CreateRequestWithPathParams).
	if idStr == "" {
		extractor := newPathParameterExtractor(r)
		return extractor.extractUUIDPathValue("id", "connector")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return uuid.Nil, common.NewValidationError("id", fmt.Sprintf("invalid connector UUID: %s", idStr))
	}
	return id, nil
}

// decodeJSON decodes a JSON request body.
func (h *ConnectorHandler) decodeJSON(r *http.Request, v interface{}) error {
	if r.Body == nil {
		return common.NewValidationError("body", "request body is required")
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(v); err != nil {
		return common.NewValidationError("body", fmt.Sprintf("invalid JSON: %v", err))
	}
	return nil
}

// writeJSON writes a JSON response.
func (h *ConnectorHandler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	if err := WriteJSON(w, statusCode, data); err != nil {
		_ = err
	}
}
