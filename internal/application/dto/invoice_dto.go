package dto

import (
	"time"

	"github.com/google/uuid"
)

type CreateInvoiceRequest struct {
	Subject         string              `json:"subject" validate:"required"`
	CustomerID      uuid.UUID           `json:"customer_id" validate:"required"`
	ContactID       *uuid.UUID          `json:"contact_id"`
	OwnerID         *uuid.UUID          `json:"owner_id"`
	InvoiceDate     time.Time           `json:"invoice_date"`
	DueDate         time.Time           `json:"due_date"`
	ReferenceNo     string              `json:"reference_no"`
	SalesOrder      string              `json:"sales_order"`
	PurchaseOrder   string              `json:"purchase_order"`
	Currency        string              `json:"currency"`
	Adjustment      float64             `json:"adjustment"`
	ExciseDuty      float64             `json:"excise_duty"`
	SalesCommission float64             `json:"sales_commission"`
	Terms           string              `json:"terms"`
	Notes           string              `json:"notes"`
	BillingStreet   string              `json:"billing_street"`
	BillingCity     string              `json:"billing_city"`
	BillingState    string              `json:"billing_state"`
	BillingCode     string              `json:"billing_code"`
	BillingCountry  string              `json:"billing_country"`
	ShippingStreet  string              `json:"shipping_street"`
	ShippingCity    string              `json:"shipping_city"`
	ShippingState   string              `json:"shipping_state"`
	ShippingCode    string              `json:"shipping_code"`
	ShippingCountry string              `json:"shipping_country"`
	Items           []CreateInvoiceItem `json:"items" validate:"required,min=1"`
}

type CreateInvoiceItem struct {
	ItemID      uuid.UUID `json:"item_id" validate:"required"`
	ItemType    string    `json:"item_type"` // Optional, defaults to service if not determined
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Quantity    float64   `json:"quantity" validate:"required,gt=0"`
	UnitPrice   float64   `json:"unit_price" validate:"required,gte=0"`
	Discount    float64   `json:"discount"`
	Tax         float64   `json:"tax"`
}

type InvoiceResponse struct {
	ID              uuid.UUID         `json:"id"`
	InvoiceNumber   string            `json:"invoice_number"`
	Subject         string            `json:"subject"`
	Status          string            `json:"status"`
	SubTotal        float64           `json:"sub_total"`
	DiscountTotal   float64           `json:"discount_total"`
	TaxTotal        float64           `json:"tax_total"`
	TotalAmount     float64           `json:"total_amount"`
	PaidAmount      float64           `json:"paid_amount"`
	BalanceAmount   float64           `json:"balance_amount"`
	Adjustment      float64           `json:"adjustment"`
	ExciseDuty      float64           `json:"excise_duty"`
	SalesCommission float64           `json:"sales_commission"`
	SalesOrder      string            `json:"sales_order"`
	PurchaseOrder   string            `json:"purchase_order"`
	OwnerID         *uuid.UUID        `json:"owner_id"`
	CustomerID      uuid.UUID         `json:"customer_id"`
	ContactID       *uuid.UUID        `json:"contact_id"`
	InvoiceDate     time.Time         `json:"invoice_date"`
	DueDate         time.Time         `json:"due_date"`
	Customer        *CustomerResponse `json:"customer,omitempty"`
	Contact         *ContactResponse  `json:"contact,omitempty"`
	Items           []ItemResponse    `json:"items,omitempty"`
	Notes           string            `json:"notes"`
	Terms           string            `json:"terms"`
	BillingStreet   string            `json:"billing_street"`
	BillingCity     string            `json:"billing_city"`
	BillingState    string            `json:"billing_state"`
	BillingCode     string            `json:"billing_code"`
	BillingCountry  string            `json:"billing_country"`
	ShippingStreet  string            `json:"shipping_street"`
	ShippingCity    string            `json:"shipping_city"`
	ShippingState   string            `json:"shipping_state"`
	ShippingCode    string            `json:"shipping_code"`
	ShippingCountry string            `json:"shipping_country"`
}

type CustomerResponse struct {
	ID          uuid.UUID `json:"id"`
	DisplayName string    `json:"display_name"`
	CompanyName string    `json:"company_name"`
}

type ContactResponse struct {
	ID        uuid.UUID `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Email     string    `json:"email"`
}

type ItemResponse struct {
	ItemID      uuid.UUID `json:"item_id"`
	ItemType    string    `json:"item_type"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Quantity    float64   `json:"quantity"`
	UnitPrice   float64   `json:"unit_price"`
	Discount    float64   `json:"discount"`
	Tax         float64   `json:"tax"`
	Total       float64   `json:"total"`
}
