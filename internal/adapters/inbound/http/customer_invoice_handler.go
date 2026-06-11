package http

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
)

// CustomerInvoiceHandler handles customer-facing invoice APIs.
// The authenticated customer's email is extracted from the X-Customer-Email header
// (propagated by KrakenD from the JWT `email` claim) and used to look up
// their associated customer record in the billing read-model.
type CustomerInvoiceHandler struct {
	invoiceService *application.InvoiceService
	paymentService *application.PaymentService
}

func NewCustomerInvoiceHandler(invoiceService *application.InvoiceService, paymentService *application.PaymentService) *CustomerInvoiceHandler {
	return &CustomerInvoiceHandler{
		invoiceService: invoiceService,
		paymentService: paymentService,
	}
}

// ListCustomerInvoices handles GET /api/v1/customer/invoices
// Returns invoices that belong to the authenticated customer.
// Customer is identified via `X-Customer-Email` header (set by KrakenD from JWT).
func (h *CustomerInvoiceHandler) ListCustomerInvoices(w http.ResponseWriter, r *http.Request) {
	customerEmail := r.Header.Get("X-Customer-Email")
	if customerEmail == "" {
		// Fallback: try the sub claim which may carry the email for customer tokens
		customerEmail = r.Header.Get("X-User-ID")
	}
	if customerEmail == "" {
		http.Error(w, "customer identity not found in token", http.StatusUnauthorized)
		return
	}

	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	// Get status filter from query params
	statusFilter := strings.ToLower(r.URL.Query().Get("status"))

	invoices, err := h.invoiceService.ListInvoicesByCustomerEmail(r.Context(), orgID, customerEmail, statusFilter)
	if err != nil {
		fmt.Printf("[ERROR] ListCustomerInvoices failed for email %s: %v\n", customerEmail, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": invoices,
	})
}

// GetCustomerInvoice handles GET /api/v1/customer/invoices/{id}
// Returns a single invoice, verifying it belongs to the authenticated customer.
func (h *CustomerInvoiceHandler) GetCustomerInvoice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid invoice ID", http.StatusBadRequest)
		return
	}

	customerEmail := r.Header.Get("X-Customer-Email")
	if customerEmail == "" {
		customerEmail = r.Header.Get("X-User-ID")
	}
	if customerEmail == "" {
		http.Error(w, "customer identity not found in token", http.StatusUnauthorized)
		return
	}

	invoice, err := h.invoiceService.GetCustomerInvoice(r.Context(), id, customerEmail)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "unauthorized") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoice)
}

// GetCustomerInvoicePayments handles GET /api/v1/customer/invoices/{id}/payments
// Returns all payments for a specific invoice, verifying it belongs to the authenticated customer.
func (h *CustomerInvoiceHandler) GetCustomerInvoicePayments(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	invoiceID, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid invoice ID", http.StatusBadRequest)
		return
	}

	customerEmail := r.Header.Get("X-Customer-Email")
	if customerEmail == "" {
		customerEmail = r.Header.Get("X-User-ID")
	}
	if customerEmail == "" {
		http.Error(w, "customer identity not found in token", http.StatusUnauthorized)
		return
	}

	// First verify the invoice belongs to the customer
	_, err = h.invoiceService.GetCustomerInvoice(r.Context(), invoiceID, customerEmail)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "unauthorized") {
			http.Error(w, "invoice not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	payments, err := h.paymentService.ListPaymentsByInvoice(r.Context(), invoiceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": payments,
	})
}

// ListCustomerPayments handles GET /api/v1/customer/payments
// Returns all payments made by the authenticated customer.
func (h *CustomerInvoiceHandler) ListCustomerPayments(w http.ResponseWriter, r *http.Request) {
	customerEmail := r.Header.Get("X-Customer-Email")
	if customerEmail == "" {
		customerEmail = r.Header.Get("X-User-ID")
	}
	if customerEmail == "" {
		http.Error(w, "customer identity not found in token", http.StatusUnauthorized)
		return
	}

	orgIDStr := r.Header.Get("X-Organization-ID")
	orgID, _ := uuid.Parse(orgIDStr)

	statusFilter := strings.ToLower(r.URL.Query().Get("status"))

	payments, err := h.paymentService.ListPaymentsByCustomerEmail(r.Context(), orgID, customerEmail, statusFilter)
	if err != nil {
		fmt.Printf("[ERROR] ListCustomerPayments failed for email %s: %v\n", customerEmail, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": payments,
	})
}

// RecordCustomerPayment handles POST /api/v1/customer/payments
// Allows a customer to record a payment intent for their invoice.
func (h *CustomerInvoiceHandler) RecordCustomerPayment(w http.ResponseWriter, r *http.Request) {
	customerEmail := r.Header.Get("X-Customer-Email")
	if customerEmail == "" {
		customerEmail = r.Header.Get("X-User-ID")
	}
	if customerEmail == "" {
		http.Error(w, "customer identity not found in token", http.StatusUnauthorized)
		return
	}

	var req dto.RecordPaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Verify the invoice belongs to the customer before allowing payment
	_, err := h.invoiceService.GetCustomerInvoice(r.Context(), req.InvoiceID, customerEmail)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "unauthorized") {
			http.Error(w, "invoice not found or does not belong to you", http.StatusForbidden)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	payment, err := h.paymentService.RecordPayment(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(payment)
}

// DownloadCustomerInvoicePDF handles GET /api/v1/customer/invoices/{id}/pdf
// Downloads the PDF for a specific invoice, verifying it belongs to the authenticated customer.
func (h *CustomerInvoiceHandler) DownloadCustomerInvoicePDF(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := uuid.Parse(vars["id"])
	if err != nil {
		http.Error(w, "invalid invoice ID", http.StatusBadRequest)
		return
	}

	customerEmail := r.Header.Get("X-Customer-Email")
	if customerEmail == "" {
		customerEmail = r.Header.Get("X-User-ID")
	}
	if customerEmail == "" {
		http.Error(w, "customer identity not found in token", http.StatusUnauthorized)
		return
	}

	// Verify the invoice belongs to the customer
	_, err = h.invoiceService.GetCustomerInvoice(r.Context(), id, customerEmail)
	if err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "unauthorized") {
			http.Error(w, "invoice not found", http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	reader, contentType, err := h.invoiceService.GetInvoicePDFStream(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, err.Error(), http.StatusNotFound)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", "attachment; filename=invoice-"+id.String()+".pdf")
	io.Copy(w, reader)
}
