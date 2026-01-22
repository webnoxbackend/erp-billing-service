	package http

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// SalesReturnHandler handles HTTP requests for sales returns
type SalesReturnHandler struct {
	service *application.SalesReturnService
}

// NewSalesReturnHandler creates a new sales return handler
func NewSalesReturnHandler(service *application.SalesReturnService) *SalesReturnHandler {
	return &SalesReturnHandler{service: service}
}

// CreateSalesReturn handles POST /api/v1/sales-returns
func (h *SalesReturnHandler) CreateSalesReturn(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateSalesReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[ERROR] Failed to decode sales return request: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get organization ID from header
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)
	req.OrganizationID = orgID

	log.Printf("[INFO] Creating sales return for order %s, org %s, items: %d", req.SalesOrderID, req.OrganizationID, len(req.Items))

	salesReturn, err := h.service.CreateSalesReturn(&req)
	if err != nil {
		log.Printf("[ERROR] Failed to create sales return: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(salesReturn)
}

// GetSalesReturn handles GET /api/v1/sales-returns/:id
func (h *SalesReturnHandler) GetSalesReturn(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales return ID", http.StatusBadRequest)
		return
	}

	salesReturn, err := h.service.GetSalesReturn(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(salesReturn)
}

// ListSalesReturns handles GET /api/v1/sales-returns
func (h *SalesReturnHandler) ListSalesReturns(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	// Parse query parameters
	filters := &dto.SalesReturnFilters{
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

	if orderID := r.URL.Query().Get("sales_order_id"); orderID != "" {
		if oid, err := uuid.Parse(orderID); err == nil {
			filters.SalesOrderID = &oid
		}
	}

	if search := r.URL.Query().Get("search"); search != "" {
		filters.Search = &search
	}

	salesReturns, total, err := h.service.ListSalesReturns(orgID, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data":      salesReturns,
		"total":     total,
		"page":      filters.Page,
		"page_size": filters.PageSize,
	})
}

// GetReturnsBySalesOrder handles GET /api/v1/billing/sales-orders/{id}/returns
func (h *SalesReturnHandler) GetReturnsBySalesOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales order ID", http.StatusBadRequest)
		return
	}

	salesReturns, err := h.service.GetReturnsBySalesOrder(orderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": salesReturns,
	})
}

// ReceiveReturn handles POST /api/v1/sales-returns/:id/receive
func (h *SalesReturnHandler) ReceiveReturn(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales return ID", http.StatusBadRequest)
		return
	}

	var req dto.ReceiveReturnRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	salesReturn, err := h.service.ReceiveReturn(id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(salesReturn)
}

// ProcessRefund handles POST /api/v1/sales-returns/:id/refund
func (h *SalesReturnHandler) ProcessRefund(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid sales return ID", http.StatusBadRequest)
		return
	}

	var req dto.ProcessRefundRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	salesReturn, err := h.service.ProcessRefund(id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(salesReturn)
}
