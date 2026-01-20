package http

import (
	"encoding/json"
	"net/http"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"strings"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

type InvoiceHandler struct {
	service *application.InvoiceService
}

func NewInvoiceHandler(service *application.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{service: service}
}

func (h *InvoiceHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	var req dto.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// In a real app, orgID would come from the auth token context
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)
	if orgID == uuid.Nil {
		// Default for testing if header is missing
		orgID = uuid.New()
	}

	invoice, err := h.service.CreateInvoice(r.Context(), orgID, req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(invoice)
}

func (h *InvoiceHandler) UpdateInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid Invoice ID", http.StatusBadRequest)
		return
	}

	var req dto.CreateInvoiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Assuming UpdateInvoice logic in service returns updated invoice or error
	invoice, err := h.service.UpdateInvoice(r.Context(), id, req)
	if err != nil {
		if err.Error() == "invoice not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(invoice)
}

func (h *InvoiceHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	// Get module filter from query params (optional)
	moduleFilter := strings.ToUpper(r.URL.Query().Get("module"))
	
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
			result, err = h.service.ListInvoices(r.Context(), orgID)
		}
		
		if sourceSystem != "" {
			result, err = h.service.ListInvoicesByModule(r.Context(), orgID, sourceSystem)
		}
	} else {
		// No filter, return all invoices
		result, err = h.service.ListInvoices(r.Context(), orgID)
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

func (h *InvoiceHandler) GetInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := uuid.Parse(vars["id"])

	invoice, err := h.service.GetInvoice(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoice)
}

func (h *InvoiceHandler) DeleteInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid Invoice ID", http.StatusBadRequest)
		return
	}

	if err := h.service.DeleteInvoice(r.Context(), id); err != nil {
		if err.Error() == "invoice not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *InvoiceHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := uuid.Parse(vars["id"])

	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// In a real app, performedBy would come from auth context
	performedBy := "System User"
	if r.Header.Get("X-User-Name") != "" {
		performedBy = r.Header.Get("X-User-Name")
	}

	err := h.service.UpdateStatus(r.Context(), id, domain.InvoiceStatus(req.Status), req.Notes, performedBy)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"message": "Status updated successfully"})
}

func (h *InvoiceHandler) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := uuid.Parse(vars["id"])

	logs, err := h.service.GetAuditLogs(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": logs,
	})
}

// SendInvoice handles sending an invoice (DRAFT → SENT)
func (h *InvoiceHandler) SendInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid Invoice ID", http.StatusBadRequest)
		return
	}

	var req dto.SendInvoiceRequest
	// For now, SendInvoiceRequest is empty, but we still decode for future fields
	if r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// Ignore decode errors for empty body
		}
	}

	invoice, err := h.service.SendInvoice(r.Context(), id, req)
	if err != nil {
		// Check for specific error types
		if err.Error() == "invoice not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if strings.HasPrefix(err.Error(), "cannot send") {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(invoice)
}

// DownloadInvoicePDF handles PDF download requests
func (h *InvoiceHandler) DownloadInvoicePDF(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid Invoice ID", http.StatusBadRequest)
		return
	}

	pdfPath, err := h.service.GetInvoicePDF(r.Context(), id)
	if err != nil {
		if err.Error() == "invoice not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if err.Error() == "invoice must be generated" { // Updated message if applicable
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Serve the PDF file
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename=invoice-"+id.String()+".pdf")
	http.ServeFile(w, r, pdfPath)
}

// PreviewInvoicePDF handles PDF preview requests (inline display)
func (h *InvoiceHandler) PreviewInvoicePDF(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "Invalid Invoice ID", http.StatusBadRequest)
		return
	}

	pdfPath, err := h.service.GetInvoicePDF(r.Context(), id)
	if err != nil {
		if err.Error() == "invoice not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else if err.Error() == "invoice must be generated" {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Serve the PDF file for inline display
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline; filename=invoice-"+id.String()+".pdf")
	http.ServeFile(w, r, pdfPath)
}
