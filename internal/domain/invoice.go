package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SourceSystem represents the origin module of an invoice
type SourceSystem string

const (
	SourceSystemFSM       SourceSystem = "FSM"
	SourceSystemCRM       SourceSystem = "CRM"
	SourceSystemInventory SourceSystem = "INVENTORY"
	SourceSystemManual    SourceSystem = "MANUAL"
)

type InvoiceStatus string

const (
	InvoiceStatusDraft   InvoiceStatus = "draft"
	InvoiceStatusSent    InvoiceStatus = "sent"
	InvoiceStatusPaid    InvoiceStatus = "paid"
	InvoiceStatusOverdue InvoiceStatus = "overdue"
	InvoiceStatusVoid    InvoiceStatus = "void"
)

type Invoice struct {
	ID              uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID  uuid.UUID     `gorm:"type:uuid;index" json:"organization_id"`
	CustomerID      uuid.UUID     `gorm:"type:uuid;index" json:"customer_id"`
	ContactID       *uuid.UUID    `gorm:"type:uuid;index" json:"contact_id"`
	OwnerID         *uuid.UUID    `gorm:"type:uuid;index" json:"owner_id"`
	Subject         string        `gorm:"type:varchar(255)" json:"subject"`
	
	// Invoice number is nullable - only generated when invoice is SENT
	InvoiceNumber   *string       `gorm:"type:varchar(50);uniqueIndex" json:"invoice_number"`
	
	// Source-agnostic fields - Billing doesn't know about FSM/CRM/Inventory internals
	// It only stores opaque references to link back to the originating module
	SourceSystem      SourceSystem `gorm:"type:varchar(20);default:'MANUAL';index" json:"source_system"`
	SourceReferenceID *string      `gorm:"type:varchar(100)" json:"source_reference_id"` // e.g., "WO-12345", "DEAL-789"
	
	ReferenceNo     string        `gorm:"type:varchar(50)" json:"reference_no"`
	SalesOrder      string        `gorm:"type:varchar(50)" json:"sales_order"`
	PurchaseOrder   string        `gorm:"type:varchar(50)" json:"purchase_order"`
	InvoiceDate     time.Time     `json:"invoice_date"`
	DueDate         time.Time     `json:"due_date"`
	Status          InvoiceStatus `gorm:"type:varchar(20);default:'draft';index" json:"status"`
	SubTotal        float64       `gorm:"type:decimal(15,2)" json:"sub_total"`
	DiscountTotal   float64       `gorm:"type:decimal(15,2)" json:"discount_total"`
	TaxTotal        float64       `gorm:"type:decimal(15,2)" json:"tax_total"`
	Adjustment      float64       `gorm:"type:decimal(15,2)" json:"adjustment"`
	ExciseDuty      float64       `gorm:"type:decimal(15,2)" json:"excise_duty"`
	SalesCommission float64       `gorm:"type:decimal(15,2)" json:"sales_commission"`
	TotalAmount     float64       `gorm:"type:decimal(15,2)" json:"total_amount"`
	PaidAmount      float64       `gorm:"type:decimal(15,2);default:0" json:"paid_amount"`
	BalanceAmount   float64       `gorm:"type:decimal(15,2)" json:"balance_amount"`
	Currency        string        `gorm:"type:varchar(3);default:'USD'" json:"currency"`
	Terms           string        `gorm:"type:text" json:"terms"`
	Notes           string        `gorm:"type:text" json:"notes"`
	
	// PDF path - populated when invoice is sent
	PDFPath         *string       `gorm:"type:varchar(500)" json:"pdf_path"`
	
	// Sales Order reference - populated when invoice is created from sales order
	SalesOrderID    *uuid.UUID    `gorm:"type:uuid;index" json:"sales_order_id"`
	
	// TDS/TCS amounts
	TDSAmount       float64       `gorm:"type:decimal(15,2);default:0" json:"tds_amount"`
	TCSAmount       float64       `gorm:"type:decimal(15,2);default:0" json:"tcs_amount"`
	
	BillingStreet   string        `gorm:"type:varchar(255)" json:"billing_street"`
	BillingCity     string        `gorm:"type:varchar(100)" json:"billing_city"`
	BillingState    string        `gorm:"type:varchar(100)" json:"billing_state"`
	BillingCode     string        `gorm:"type:varchar(20)" json:"billing_code"`
	BillingCountry  string        `gorm:"type:varchar(100)" json:"billing_country"`
	ShippingStreet  string        `gorm:"type:varchar(255)" json:"shipping_street"`
	ShippingCity    string        `gorm:"type:varchar(100)" json:"shipping_city"`
	ShippingState   string        `gorm:"type:varchar(100)" json:"shipping_state"`
	ShippingCode    string        `gorm:"type:varchar(20)" json:"shipping_code"`
	ShippingCountry string        `gorm:"type:varchar(100)" json:"shipping_country"`
	Items           []InvoiceItem `gorm:"foreignKey:InvoiceID" json:"items"`
	Payments        []Payment     `gorm:"foreignKey:InvoiceID" json:"payments"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	DeletedAt       *time.Time    `gorm:"index" json:"deleted_at,omitempty"`
}

// CanEdit returns true if the invoice can be edited (only DRAFT invoices)
func (i *Invoice) CanEdit() bool {
	return i.Status == InvoiceStatusDraft
}

// CanSend returns true if the invoice can be sent (only DRAFT invoices)
func (i *Invoice) CanSend() bool {
	return i.Status == InvoiceStatusDraft
}

// CanTransitionTo validates if the invoice can transition to the new status
func (i *Invoice) CanTransitionTo(newStatus InvoiceStatus) error {
	// Define valid transitions
	validTransitions := map[InvoiceStatus][]InvoiceStatus{
		InvoiceStatusDraft:   {InvoiceStatusSent, InvoiceStatusVoid},
		InvoiceStatusSent:    {InvoiceStatusPaid, InvoiceStatusOverdue, InvoiceStatusVoid},
		InvoiceStatusOverdue: {InvoiceStatusPaid, InvoiceStatusVoid},
		InvoiceStatusPaid:    {InvoiceStatusVoid}, // Can void a paid invoice (refund scenario)
		InvoiceStatusVoid:    {},                  // Terminal state
	}

	allowedStatuses, exists := validTransitions[i.Status]
	if !exists {
		return fmt.Errorf("unknown current status: %s", i.Status)
	}

	for _, allowed := range allowedStatuses {
		if allowed == newStatus {
			return nil
		}
	}

	return fmt.Errorf("cannot transition from %s to %s", i.Status, newStatus)
}

// CanReceivePayment returns true if the invoice can accept payments
// Only SENT invoices can receive payments
func (i *Invoice) CanReceivePayment() bool {
	return i.Status == InvoiceStatusSent
}

// CanRefund returns true if the invoice can be refunded
// Only PAID invoices can be refunded
func (i *Invoice) CanRefund() bool {
	return i.Status == InvoiceStatusPaid
}

// CalculateStatus derives the invoice status based on payment amounts
// This ensures invoice status is always consistent with payment state
func (i *Invoice) CalculateStatus() InvoiceStatus {
	// If fully paid, status is PAID
	if i.BalanceAmount == 0 && i.PaidAmount > 0 {
		return InvoiceStatusPaid
	}
	// Otherwise keep current status (DRAFT or SENT)
	// Partial payments don't change the status - it remains SENT
	return i.Status
}

type InvoiceItem struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	InvoiceID   uuid.UUID `gorm:"type:uuid;index" json:"invoice_id"`
	ItemID      uuid.UUID `gorm:"type:uuid;index" json:"item_id"`                      // Reference to Service/Part Read Model
	ItemType    string    `gorm:"type:varchar(20);default:'service'" json:"item_type"` // 'service' or 'part'
	Name        string    `gorm:"type:varchar(255)" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Quantity    float64   `gorm:"type:decimal(15,2)" json:"quantity"`
	UnitPrice   float64   `gorm:"type:decimal(15,2)" json:"unit_price"`
	Discount    float64   `gorm:"type:decimal(15,2)" json:"discount"`
	Tax         float64   `gorm:"type:decimal(15,2)" json:"tax"`
	Total       float64   `gorm:"type:decimal(15,2)" json:"total"`
	
	// Metadata stores module-specific data without schema changes
	// Examples:
	// - FSM: {"technician_id": "TECH-001", "service_hours": 2.5}
	// - CRM: {"deal_id": "DEAL-123", "sales_rep": "REP-456"}
	// - Inventory: {"warehouse": "WH-01", "batch_number": "BATCH-789"}
	// Billing service stores and returns this data but never interprets it
	Metadata    json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Payment represents a payment made against an invoice
type Payment struct {
	ID             uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID     `gorm:"type:uuid;index" json:"organization_id"`
	InvoiceID      uuid.UUID     `gorm:"type:uuid;index" json:"invoice_id"`
	Amount         float64       `gorm:"type:decimal(15,2)" json:"amount"`
	PaymentDate    time.Time     `json:"payment_date"`
	Method         PaymentMethod `gorm:"type:varchar(50);column:payment_method" json:"method"`
	Reference      string        `gorm:"type:varchar(100);column:transaction_ref" json:"reference"`
	Status         PaymentStatus `gorm:"type:varchar(20);default:'completed'" json:"status"`
	PaymentType    PaymentType   `gorm:"type:varchar(20);default:'payment'" json:"payment_type"`
	SalesReturnID  *uuid.UUID    `gorm:"type:uuid;index" json:"sales_return_id"`
	Notes          string        `gorm:"type:text" json:"notes"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

// PaymentType represents the type of payment
type PaymentType string

const (
	PaymentTypePayment PaymentType = "payment"
	PaymentTypeRefund  PaymentType = "refund"
)

// PaymentMethod represents the payment method
type PaymentMethod string

const (
	PaymentMethodCash   PaymentMethod = "cash"
	PaymentMethodBank   PaymentMethod = "bank"
	PaymentMethodUPI    PaymentMethod = "upi"
	PaymentMethodCheque PaymentMethod = "cheque"
	PaymentMethodOther  PaymentMethod = "other"
)

// PaymentStatus represents the payment status
type PaymentStatus string

const (
	PaymentStatusCompleted PaymentStatus = "completed"
	PaymentStatusVoid      PaymentStatus = "void"
)

// CanVoid returns true if the payment can be voided
func (p *Payment) CanVoid() bool {
	return p.Status == PaymentStatusCompleted
}

type InvoiceAuditLog struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;index" json:"organization_id"`
	InvoiceID      uuid.UUID `gorm:"type:uuid;index" json:"invoice_id"`
	Action         string    `gorm:"type:varchar(100)" json:"action"`
	OldStatus      string    `gorm:"type:varchar(50)" json:"old_status"`
	NewStatus      string    `gorm:"type:varchar(50)" json:"new_status"`
	Notes          string    `gorm:"type:text" json:"notes"`
	PerformedBy    string    `gorm:"type:varchar(255)" json:"performed_by"`
	CreatedAt      time.Time `json:"created_at"`
}
