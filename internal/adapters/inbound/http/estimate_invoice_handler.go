package http

import (
	"encoding/json"
	"net/http"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type EstimateInvoiceHandler struct {
	invoiceService *application.InvoiceService
}

func NewEstimateInvoiceHandler(invoiceService *application.InvoiceService) *EstimateInvoiceHandler {
	return &EstimateInvoiceHandler{
		invoiceService: invoiceService,
	}
}

// CreateInvoiceFromEstimate handles POST /billing/estimates/{id}/invoice
func (h *EstimateInvoiceHandler) CreateInvoiceFromEstimate(w http.ResponseWriter, r *http.Request) {
	// Get organization ID from header
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		http.Error(w, "invalid organization ID", http.StatusBadRequest)
		return
	}

	// Get estimate ID from URL
	vars := mux.Vars(r)
	estimateID := vars["id"]
	if estimateID == "" {
		http.Error(w, "estimate ID is required", http.StatusBadRequest)
		return
	}

	// Decode request body
	var req dto.CreateInvoiceFromEstimateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Set estimate ID from URL
	req.EstimateID = estimateID

	// Create invoice from estimate
	invoice, err := h.invoiceService.CreateInvoiceFromEstimate(r.Context(), orgID, req)
	if err != nil {
		http.Error(w, "failed to create invoice: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return created invoice
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(invoice)
}
