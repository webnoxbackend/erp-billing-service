package http

import (
	"encoding/json"
	"net/http"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
)

type ReadModelHandler struct {
	repo domain.ReadModelRepository
}

func NewReadModelHandler(repo domain.ReadModelRepository) *ReadModelHandler {
	return &ReadModelHandler{repo: repo}
}

func (h *ReadModelHandler) SearchCustomers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	res, err := h.repo.SearchCustomers(r.Context(), orgID, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": res,
	})
}

func (h *ReadModelHandler) SearchItems(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	res, err := h.repo.SearchItems(r.Context(), orgID, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": res,
	})
}

func (h *ReadModelHandler) SearchContacts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	customerIDStr := r.URL.Query().Get("customer_id")
	customerID, _ := uuid.Parse(customerIDStr)

	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	res, err := h.repo.SearchContacts(r.Context(), orgID, customerID, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": res,
	})
}
