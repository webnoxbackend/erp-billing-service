package dto

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Sales Order DTOs
// ============================================================================

// CreateSalesOrderRequest represents a request to create a new sales order
type CreateSalesOrderRequest struct {
	OrganizationID uuid.UUID           `json:"organization_id" validate:"required"`
	CustomerID     uuid.UUID           `json:"customer_id" validate:"required"`
	ContactID      *uuid.UUID          `json:"contact_id,omitempty"`
	OrderDate      time.Time           `json:"order_date" validate:"required"`
	Items          []SalesOrderItemDTO `json:"items" validate:"required,min=1,dive"`
	TDSAmount      float64             `json:"tds_amount"`
	TCSAmount      float64             `json:"tcs_amount"`
	Terms          string              `json:"terms"`
	Notes          string              `json:"notes"`
}

// UpdateSalesOrderRequest represents a request to update a sales order
type UpdateSalesOrderRequest struct {
	CustomerID *uuid.UUID          `json:"customer_id,omitempty"`
	ContactID  *uuid.UUID          `json:"contact_id,omitempty"`
	OrderDate  *time.Time          `json:"order_date,omitempty"`
	Items      []SalesOrderItemDTO `json:"items,omitempty"`
	TDSAmount  *float64            `json:"tds_amount,omitempty"`
	TCSAmount  *float64            `json:"tcs_amount,omitempty"`
	Terms      *string             `json:"terms,omitempty"`
	Notes      *string             `json:"notes,omitempty"`
}

// SalesOrderItemDTO represents a line item in a sales order
type SalesOrderItemDTO struct {
	ID          *uuid.UUID      `json:"id,omitempty"`
	ItemID      uuid.UUID       `json:"item_id" validate:"required"`
	ItemType    string          `json:"item_type"`
	Name        string          `json:"name" validate:"required"`
	Description string          `json:"description"`
	Quantity    float64         `json:"quantity" validate:"required,gt=0"`
	UnitPrice   float64         `json:"unit_price" validate:"required,gte=0"`
	Discount    float64         `json:"discount"`
	Tax         float64         `json:"tax"`
	Total       float64         `json:"total"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// SalesOrderResponse represents a sales order response
type SalesOrderResponse struct {
	ID             uuid.UUID           `json:"id"`
	OrganizationID uuid.UUID           `json:"organization_id"`
	CustomerID     uuid.UUID           `json:"customer_id"`
	ContactID      *uuid.UUID          `json:"contact_id,omitempty"`
	OrderNumber    *string             `json:"order_number,omitempty"`
	OrderDate      time.Time           `json:"order_date"`
	Status         string              `json:"status"`
	SubTotal       float64             `json:"sub_total"`
	DiscountTotal  float64             `json:"discount_total"`
	TaxTotal       float64             `json:"tax_total"`
	TDSAmount      float64             `json:"tds_amount"`
	TCSAmount      float64             `json:"tcs_amount"`
	TotalAmount    float64             `json:"total_amount"`
	InvoiceID      *uuid.UUID          `json:"invoice_id,omitempty"`
	ShippedDate    *time.Time          `json:"shipped_date,omitempty"`
	Terms          string              `json:"terms"`
	Notes          string              `json:"notes"`
	Items          []SalesOrderItemDTO `json:"items"`
	CreatedAt      time.Time           `json:"created_at"`
	UpdatedAt      time.Time           `json:"updated_at"`
}

// SalesOrderFilters represents filters for listing sales orders
type SalesOrderFilters struct {
	Status     *string    `json:"status,omitempty"`
	CustomerID *uuid.UUID `json:"customer_id,omitempty"`
	FromDate   *time.Time `json:"from_date,omitempty"`
	ToDate     *time.Time `json:"to_date,omitempty"`
	Search     *string    `json:"search,omitempty"` // Search by order number or customer name
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
}

// MarkAsShippedRequest represents a request to mark an order as shipped
type MarkAsShippedRequest struct {
	ShippedDate time.Time `json:"shipped_date" validate:"required"`
	Notes       string    `json:"notes"`
}

// ============================================================================
// Sales Return DTOs
// ============================================================================

// CreateSalesReturnRequest represents a request to create a new sales return
type CreateSalesReturnRequest struct {
	OrganizationID uuid.UUID            `json:"organization_id" validate:"required"`
	SalesOrderID   uuid.UUID            `json:"sales_order_id" validate:"required"`
	ReturnDate     time.Time            `json:"return_date" validate:"required"`
	ReturnReason   string               `json:"return_reason" validate:"required"`
	Items          []SalesReturnItemDTO `json:"items" validate:"required,min=1,dive"`
	Notes          string               `json:"notes"`
}

// SalesReturnItemDTO represents a line item in a sales return
type SalesReturnItemDTO struct {
	ID               *uuid.UUID `json:"id,omitempty"`
	SalesOrderItemID uuid.UUID  `json:"sales_order_item_id" validate:"required"`
	ReturnedQuantity float64    `json:"returned_quantity" validate:"required,gt=0"`
	UnitPrice        float64    `json:"unit_price" validate:"required,gte=0"`
	Tax              float64    `json:"tax"`
	Total            float64    `json:"total"`
	Reason           string     `json:"reason"`
}

// SalesReturnResponse represents a sales return response
type SalesReturnResponse struct {
	ID              uuid.UUID            `json:"id"`
	OrganizationID  uuid.UUID            `json:"organization_id"`
	SalesOrderID    uuid.UUID            `json:"sales_order_id"`
	ReturnNumber    *string              `json:"return_number,omitempty"`
	ReturnDate      time.Time            `json:"return_date"`
	Status          string               `json:"status"`
	ReturnAmount    float64              `json:"return_amount"`
	ReturnReason    string               `json:"return_reason"`
	Notes           string               `json:"notes"`
	ApprovedDate    *time.Time           `json:"approved_date,omitempty"`
	ReceivedDate    *time.Time           `json:"received_date,omitempty"`
	ReceivingNotes  string               `json:"receiving_notes"`
	RefundedDate    *time.Time           `json:"refunded_date,omitempty"`
	RefundPaymentID *uuid.UUID           `json:"refund_payment_id,omitempty"`
	Items           []SalesReturnItemDTO `json:"items"`
	CreatedAt       time.Time            `json:"created_at"`
	UpdatedAt       time.Time            `json:"updated_at"`
}

// SalesReturnFilters represents filters for listing sales returns
type SalesReturnFilters struct {
	Status       *string    `json:"status,omitempty"`
	SalesOrderID *uuid.UUID `json:"sales_order_id,omitempty"`
	FromDate     *time.Time `json:"from_date,omitempty"`
	ToDate       *time.Time `json:"to_date,omitempty"`
	Search       *string    `json:"search,omitempty"` // Search by return number
	Page         int        `json:"page"`
	PageSize     int        `json:"page_size"`
}

// ReceiveReturnRequest represents a request to receive returned items
type ReceiveReturnRequest struct {
	ReceivedDate   time.Time `json:"received_date" validate:"required"`
	ReceivingNotes string    `json:"receiving_notes"`
}

// ProcessRefundRequest represents a request to process a refund
type ProcessRefundRequest struct {
	PaymentDate time.Time `json:"payment_date" validate:"required"`
	Method      string    `json:"method" validate:"required"`
	Reference   string    `json:"reference"`
	Notes       string    `json:"notes"`
}
