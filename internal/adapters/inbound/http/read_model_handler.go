package http

import (
	"encoding/json"
	"net/http"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
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

	for i := range res {
		res[i].UpdatedAt = application.ConvertToOrgTZValue(r.Context(), res[i].UpdatedAt, orgID, h.repo)
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

	for i := range res {
		res[i].UpdatedAt = application.ConvertToOrgTZValue(r.Context(), res[i].UpdatedAt, orgID, h.repo)
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

	for i := range res {
		res[i].UpdatedAt = application.ConvertToOrgTZValue(r.Context(), res[i].UpdatedAt, orgID, h.repo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": res,
	})
}

// SearchWorkOrders handles GET /api/v1/billing/work-orders?search=...
// Returns work orders from the local work_order_rms table (Kafka-synced replica).
func (h *ReadModelHandler) SearchWorkOrders(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("search")
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	res, err := h.repo.SearchWorkOrders(r.Context(), orgID, query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for i := range res {
		res[i].UpdatedAt = application.ConvertToOrgTZValue(r.Context(), res[i].UpdatedAt, orgID, h.repo)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": res,
	})
}

// GetWorkOrder handles GET /api/v1/billing/work-orders/{id}
// Returns a single work order with its service and part lines preloaded.
func (h *ReadModelHandler) GetWorkOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid work order ID", http.StatusBadRequest)
		return
	}

	res, err := h.repo.GetWorkOrderByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	res.UpdatedAt = application.ConvertToOrgTZValue(r.Context(), res.UpdatedAt, res.OrganizationID, h.repo)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": res,
	})
}
