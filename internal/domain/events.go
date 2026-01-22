package domain

import "time"

// Event represents a domain event
type Event struct {
	Type      string
	Payload   interface{}
	Timestamp time.Time
}

// InvoiceCreatedEvent represents an invoice creation event
// Emitted when an invoice is first created (DRAFT status)
type InvoiceCreatedEvent struct {
	InvoiceID         string    `json:"invoice_id"`
	OrganizationID    string    `json:"organization_id"`
	CustomerID        string    `json:"customer_id"`
	SourceSystem      string    `json:"source_system"`       // FSM, CRM, INVENTORY, MANUAL
	SourceReferenceID *string   `json:"source_reference_id"` // Opaque reference to originating module
	Subject           string    `json:"subject"`
	Status            string    `json:"status"` // Always "draft" on creation
	TotalAmount       float64   `json:"total_amount"`
	Currency          string    `json:"currency"`
	Timestamp         time.Time `json:"timestamp"`
}

// InvoiceSentEvent represents an invoice being sent to customer
// Emitted when invoice transitions from DRAFT to SENT
type InvoiceSentEvent struct {
	InvoiceID         string    `json:"invoice_id"`
	InvoiceNumber     string    `json:"invoice_number"` // Generated on send
	OrganizationID    string    `json:"organization_id"`
	CustomerID        string    `json:"customer_id"`
	SourceSystem      string    `json:"source_system"`
	SourceReferenceID *string   `json:"source_reference_id"`
	PDFPath           *string   `json:"pdf_path"` // Path to generated PDF
	TotalAmount       float64   `json:"total_amount"`
	SentAt            time.Time `json:"sent_at"`
	Timestamp         time.Time `json:"timestamp"`
}

// Example events (for backward compatibility with example service)
type ExampleCreatedEvent struct {
	ExampleID int64
	Name      string
	Timestamp time.Time
}

type ExampleUpdatedEvent struct {
	ExampleID int64
	Name      string
	Timestamp time.Time
}

type ExampleDeletedEvent struct {
	ExampleID int64
	Timestamp time.Time
}

// PaymentReceivedEvent represents a payment being recorded
type PaymentReceivedEvent struct {
	PaymentID      string    `json:"payment_id"`
	InvoiceID      string    `json:"invoice_id"`
	OrganizationID string    `json:"organization_id"`
	Amount         float64   `json:"amount"`
	Method         string    `json:"method"`
	PaymentDate    time.Time `json:"payment_date"`
	Timestamp      time.Time `json:"timestamp"`
}

// PaymentRecordedEvent represents a payment being recorded with full context
// Used to update source systems (e.g., work orders) about payment status
type PaymentRecordedEvent struct {
	PaymentID         string    `json:"payment_id"`
	InvoiceID         string    `json:"invoice_id"`
	OrganizationID    string    `json:"organization_id"`
	Amount            float64   `json:"amount"`
	Method            string    `json:"method"`
	PaymentType       string    `json:"payment_type"` // "payment" or "refund"
	PaymentDate       time.Time `json:"payment_date"`
	InvoiceTotal      float64   `json:"invoice_total"`
	TotalPaid         float64   `json:"total_paid"`
	BalanceDue        float64   `json:"balance_due"`
	SourceSystem      string    `json:"source_system"`
	SourceReferenceID *string   `json:"source_reference_id"`
	Timestamp         time.Time `json:"timestamp"`
}

// InvoicePartiallyPaidEvent represents an invoice being partially paid
type InvoicePartiallyPaidEvent struct {
	InvoiceID         string    `json:"invoice_id"`
	OrganizationID    string    `json:"organization_id"`
	SourceSystem      string    `json:"source_system"`
	SourceReferenceID *string   `json:"source_reference_id"`
	PaidAmount        float64   `json:"paid_amount"`
	BalanceDue        float64   `json:"balance_due"`
	Timestamp         time.Time `json:"timestamp"`
}

// InvoicePaidEvent represents an invoice being fully paid
type InvoicePaidEvent struct {
	InvoiceID         string    `json:"invoice_id"`
	OrganizationID    string    `json:"organization_id"`
	SourceSystem      string    `json:"source_system"`
	SourceReferenceID *string   `json:"source_reference_id"`
	PaidAt            time.Time `json:"paid_at"`
	TotalAmount       float64   `json:"total_amount"`
	Timestamp         time.Time `json:"timestamp"`
}

// BaseEvent provides common fields for all events
type BaseEvent struct {
	EventID        string    `json:"event_id"`
	EventType      string    `json:"event_type"`
	AggregateID    string    `json:"aggregate_id"`
	AggregateType  string    `json:"aggregate_type"`
	OrganizationID string    `json:"organization_id"`
	Timestamp      time.Time `json:"timestamp"`
}

// ============================================================================
// Sales Order Events
// ============================================================================

// SalesOrderCreatedEvent represents a sales order creation event
type SalesOrderCreatedEvent struct {
	SalesOrderID string   `json:"sales_order_id"`
	OrderNumber  *string  `json:"order_number,omitempty"`
	CustomerID   string   `json:"customer_id"`
	TotalAmount  float64  `json:"total_amount"`
	Status       string   `json:"status"`
}

// SalesOrderUpdatedEvent represents a sales order update event
type SalesOrderUpdatedEvent struct {
	SalesOrderID string `json:"sales_order_id"`
}

// SalesOrderConfirmedEvent represents a sales order confirmation event
type SalesOrderConfirmedEvent struct {
	SalesOrderID string `json:"sales_order_id"`
	OrderNumber  string `json:"order_number"`
}

// SalesOrderInvoicedEvent represents a sales order being invoiced
type SalesOrderInvoicedEvent struct {
	SalesOrderID string `json:"sales_order_id"`
	InvoiceID    string `json:"invoice_id"`
}

// SalesOrderShippedEvent represents a sales order being shipped
type SalesOrderShippedEvent struct {
	SalesOrderID string    `json:"sales_order_id"`
	ShippedDate  time.Time `json:"shipped_date"`
}

// SalesOrderCancelledEvent represents a sales order cancellation
type SalesOrderCancelledEvent struct {
	SalesOrderID string `json:"sales_order_id"`
	Reason       string `json:"reason"`
}

// ============================================================================
// Sales Return Events
// ============================================================================

// SalesReturnCreatedEvent represents a sales return creation event
type SalesReturnCreatedEvent struct {
	SalesReturnID string  `json:"sales_return_id"`
	ReturnNumber  string  `json:"return_number"`
	SalesOrderID  string  `json:"sales_order_id"`
	ReturnAmount  float64 `json:"return_amount"`
}

// SalesReturnReceivedEvent represents a sales return being received
type SalesReturnReceivedEvent struct {
	SalesReturnID string    `json:"sales_return_id"`
	ReceivedDate  time.Time `json:"received_date"`
}

// SalesReturnRefundedEvent represents a sales return being refunded
type SalesReturnRefundedEvent struct {
	SalesReturnID string  `json:"sales_return_id"`
	RefundAmount  float64 `json:"refund_amount"`
	PaymentID     string  `json:"payment_id"`
}

