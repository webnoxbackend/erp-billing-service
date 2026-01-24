package dto

import (
	"time"

	"github.com/google/uuid"
)

// CreateInvoiceFromEstimateRequest represents the request to convert an estimate to an invoice
type CreateInvoiceFromEstimateRequest struct {
	EstimateID     string     `json:"estimate_id"`
	CustomerID     uuid.UUID  `json:"customer_id"`
	ContactID      *uuid.UUID `json:"contact_id,omitempty"`
	Subject        string     `json:"subject"`
	InvoiceDate    time.Time  `json:"invoice_date"`
	DueDate        time.Time  `json:"due_date"`
	Currency       string     `json:"currency"`
	
	// Totals from estimate
	SubTotal       float64    `json:"sub_total"`
	Discount       float64    `json:"discount"`
	Adjustment     float64    `json:"adjustment"`
	TotalAmount    float64    `json:"total_amount"`
	
	// Address fields
	BillingStreet  string     `json:"billing_street,omitempty"`
	BillingCity    string     `json:"billing_city,omitempty"`
	BillingState   string     `json:"billing_state,omitempty"`
	BillingCode    string     `json:"billing_code,omitempty"`
	BillingCountry string     `json:"billing_country,omitempty"`
	ShippingStreet string     `json:"shipping_street,omitempty"`
	ShippingCity   string     `json:"shipping_city,omitempty"`
	ShippingState  string     `json:"shipping_state,omitempty"`
	ShippingCode   string     `json:"shipping_code,omitempty"`
	ShippingCountry string    `json:"shipping_country,omitempty"`
	
	// Line items
	Items          []EstimateItemDTO `json:"items"`
	Terms          string            `json:"terms,omitempty"`
	Notes          string            `json:"notes,omitempty"`
}

// EstimateItemDTO represents a line item from an estimate
type EstimateItemDTO struct {
	ItemID      string  `json:"item_id"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"`
	Tax         float64 `json:"tax"`
	Discount    float64 `json:"discount"`
	Total       float64 `json:"total"`
}
