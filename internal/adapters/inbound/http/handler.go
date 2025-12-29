package http

import (
	"encoding/json"
	"example-service/internal/application/dto"
	"example-service/internal/ports/services"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// Handler implements the HTTP handler
type Handler struct {
	exampleService services.ExampleService
}

// NewHandler creates a new HTTP handler
func NewHandler(exampleService services.ExampleService) *Handler {
	return &Handler{
		exampleService: exampleService,
	}
}

// RegisterRoutes registers all HTTP routes
func (h *Handler) RegisterRoutes(router *mux.Router) {
	router.HandleFunc("/api/v1/examples", h.CreateExample).Methods("POST")
	router.HandleFunc("/api/v1/examples/{id}", h.GetExample).Methods("GET")
	router.HandleFunc("/api/v1/examples", h.ListExamples).Methods("GET")
	router.HandleFunc("/api/v1/examples/{id}", h.UpdateExample).Methods("PUT")
	router.HandleFunc("/api/v1/examples/{id}", h.DeleteExample).Methods("DELETE")
}

// CreateExample handles POST /api/v1/examples
func (h *Handler) CreateExample(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateExampleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.exampleService.CreateExample(&req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// GetExample handles GET /api/v1/examples/{id}
func (h *Handler) GetExample(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	resp, err := h.exampleService.GetExample(id)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// ListExamples handles GET /api/v1/examples
func (h *Handler) ListExamples(w http.ResponseWriter, r *http.Request) {
	examples, err := h.exampleService.ListExamples()
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(examples)
}

// UpdateExample handles PUT /api/v1/examples/{id}
func (h *Handler) UpdateExample(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	var req dto.UpdateExampleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.exampleService.UpdateExample(id, &req)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// DeleteExample handles DELETE /api/v1/examples/{id}
func (h *Handler) DeleteExample(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.ParseInt(vars["id"], 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	if err := h.exampleService.DeleteExample(id); err != nil {
		h.handleError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleError handles errors and returns appropriate HTTP responses
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	// In a real implementation, you would map domain errors to HTTP status codes
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

