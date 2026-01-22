package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// SalesOrderHandler handles HTTP requests for sales orders
type SalesOrderHandler struct {
	service *application.SalesOrderService
}

// NewSalesOrderHandler creates a new sales order handler
func NewSalesOrderHandler(service *application.SalesOrderService) *SalesOrderHandler {
	return &SalesOrderHandler{service: service}
}

// CreateSalesOrder handles POST /api/v1/sales-orders
func (h *SalesOrderHandler) CreateSalesOrder(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateSalesOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get organization ID from header
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)
	req.OrganizationID = orgID

	order, err := h.service.CreateSalesOrder(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

// UpdateSalesOrder handles PUT /api/v1/sales-orders/:id
func (h *SalesOrderHandler) UpdateSalesOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales order ID", http.StatusBadRequest)
		return
	}

	var req dto.UpdateSalesOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	order, err := h.service.UpdateSalesOrder(id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// GetSalesOrder handles GET /api/v1/sales-orders/:id
func (h *SalesOrderHandler) GetSalesOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales order ID", http.StatusBadRequest)
		return
	}

	order, err := h.service.GetSalesOrder(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// ListSalesOrders handles GET /api/v1/sales-orders
func (h *SalesOrderHandler) ListSalesOrders(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	// Parse query parameters
	filters := &dto.SalesOrderFilters{
		Page:     1,
		PageSize: 20,
	}

	if page := r.URL.Query().Get("page"); page != "" {
		if p, err := strconv.Atoi(page); err == nil {
			filters.Page = p
		}
	}

	if pageSize := r.URL.Query().Get("page_size"); pageSize != "" {
		if ps, err := strconv.Atoi(pageSize); err == nil {
			filters.PageSize = ps
		}
	}

	if status := r.URL.Query().Get("status"); status != "" {
		filters.Status = &status
	}

	if customerID := r.URL.Query().Get("customer_id"); customerID != "" {
		if cid, err := uuid.Parse(customerID); err == nil {
			filters.CustomerID = &cid
		}
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filters.Search = &search
	}

	orders, total, err := h.service.ListSalesOrders(orgID, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":  orders,
		"total": total,
		"page":  filters.Page,
		"page_size": filters.PageSize,
	})
}

// ConfirmSalesOrder handles POST /api/v1/sales-orders/:id/confirm
func (h *SalesOrderHandler) ConfirmSalesOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales order ID", http.StatusBadRequest)
		return
	}

	order, err := h.service.ConfirmSalesOrder(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// CreateInvoiceFromOrder handles POST /api/v1/sales-orders/:id/create-invoice
func (h *SalesOrderHandler) CreateInvoiceFromOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales order ID", http.StatusBadRequest)
		return
	}

	invoice, err := h.service.CreateInvoiceFromOrder(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(invoice)
}

// MarkAsShipped handles POST /api/v1/sales-orders/:id/ship
func (h *SalesOrderHandler) MarkAsShipped(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales order ID", http.StatusBadRequest)
		return
	}

	var req dto.MarkAsShippedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	order, err := h.service.MarkAsShipped(id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// CancelSalesOrder handles DELETE /api/v1/sales-orders/:id
func (h *SalesOrderHandler) CancelSalesOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales order ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, use default reason
		req.Reason = "Cancelled by user"
	}

	if err := h.service.CancelSalesOrder(id, req.Reason); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
