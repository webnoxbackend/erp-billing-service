package dto

import (
	"time"

	"github.com/google/uuid"
)

// RecordPaymentRequest represents a request to record a payment
type RecordPaymentRequest struct {
	InvoiceID   uuid.UUID `json:"invoice_id" validate:"required"`
	Amount      float64   `json:"amount" validate:"required,gt=0"`
	Method      string    `json:"method" validate:"required,oneof=cash bank upi cheque other"`
	Reference   string    `json:"reference"`
	PaymentDate string    `json:"payment_date" validate:"required"`
	Notes       string    `json:"notes"`
}

// PaymentResponse represents a payment response
type PaymentResponse struct {
	ID          string    `json:"id"`
	InvoiceID   string    `json:"invoice_id"`
	Amount      float64   `json:"amount"`
	Method      string    `json:"method"`
	Reference   string    `json:"reference"`
	PaymentDate time.Time `json:"payment_date"`
	Status      string    `json:"status"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
}

// VoidPaymentRequest represents a request to void a payment
type VoidPaymentRequest struct {
	Notes string `json:"notes"`
}
