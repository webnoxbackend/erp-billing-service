package http

import (
	"encoding/json"
	"net/http"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type PaymentHandler struct {
	service *application.PaymentService
}

func NewPaymentHandler(service *application.PaymentService) *PaymentHandler {
	return &PaymentHandler{service: service}
}

// RecordPayment handles POST /api/v1/billing/payments
func (h *PaymentHandler) RecordPayment(w http.ResponseWriter, r *http.Request) {
	var req dto.RecordPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	payment, err := h.service.RecordPayment(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(payment)
}

// VoidPayment handles POST /api/v1/billing/payments/{id}/void
func (h *PaymentHandler) VoidPayment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paymentID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid payment ID", http.StatusBadRequest)
		return
	}

	var req dto.VoidPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := h.service.VoidPayment(r.Context(), paymentID, req.Notes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListPayments handles GET /api/v1/billing/payments
func (h *PaymentHandler) ListPayments(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)
	
	// Get module filter from query params (optional)
	moduleFilter := r.URL.Query().Get("module")
	
	var err error
	var result interface{}
	
	if moduleFilter != "" {
		// Filter by specific module (FSM, CRM, INVENTORY)
		var sourceSystem domain.SourceSystem
		switch moduleFilter {
		case "FSM":
			sourceSystem = domain.SourceSystemFSM
		case "CRM":
			sourceSystem = domain.SourceSystemCRM
		case "INVENTORY", "IMS":
			sourceSystem = domain.SourceSystemInventory
		default:
			// Invalid module, return all
			result, err = h.service.ListAllPayments(r.Context())
		}
		
		if sourceSystem != "" {
			result, err = h.service.ListPaymentsByModule(r.Context(), orgID, sourceSystem)
		}
	} else {
		// No filter, return all payments
		result, err = h.service.ListAllPayments(r.Context())
	}
	
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": result,
	})
}

// ListPaymentsByInvoice handles GET /api/v1/billing/invoices/{id}/payments
func (h *PaymentHandler) ListPaymentsByInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	invoiceID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid invoice ID", http.StatusBadRequest)
		return
	}

	payments, err := h.service.ListPaymentsByInvoice(r.Context(), invoiceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": payments,
	})
}

// GetPayment handles GET /api/v1/billing/payments/{id}
func (h *PaymentHandler) GetPayment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paymentID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid payment ID", http.StatusBadRequest)
		return
	}

	payment, err := h.service.GetPayment(r.Context(), paymentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}
