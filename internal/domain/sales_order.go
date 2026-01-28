package domain

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SalesOrderStatus represents the status of a sales order
type SalesOrderStatus string

const (
	SalesOrderStatusDraft         SalesOrderStatus = "draft"
	SalesOrderStatusConfirmed     SalesOrderStatus = "confirmed"
	SalesOrderStatusInvoiced      SalesOrderStatus = "invoiced"
	SalesOrderStatusPartiallyPaid SalesOrderStatus = "partially_paid"
	SalesOrderStatusPaid          SalesOrderStatus = "paid"
	SalesOrderStatusShipped       SalesOrderStatus = "shipped"
	SalesOrderStatusCompleted     SalesOrderStatus = "completed"
	SalesOrderStatusCancelled     SalesOrderStatus = "cancelled"
)

// SalesOrder represents a customer sales order
type SalesOrder struct {
	ID             uuid.UUID        `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID        `gorm:"type:uuid;index" json:"organization_id"`
	CustomerID     uuid.UUID        `gorm:"type:uuid;index" json:"customer_id"`
	ContactID      *uuid.UUID       `gorm:"type:uuid;index" json:"contact_id"`
	OrderNumber    *string          `gorm:"type:varchar(50);unique" json:"order_number"`
	OrderDate      time.Time        `json:"order_date"`
	Status         SalesOrderStatus `gorm:"type:varchar(20);default:'draft';index" json:"status"`
	
	// Financial fields
	SubTotal      float64 `gorm:"type:decimal(15,2)" json:"sub_total"`
	DiscountTotal float64 `gorm:"type:decimal(15,2);default:0" json:"discount_total"`
	TaxTotal      float64 `gorm:"type:decimal(15,2);default:0" json:"tax_total"`
	TDSAmount     float64 `gorm:"type:decimal(15,2);default:0" json:"tds_amount"`
	TCSAmount     float64 `gorm:"type:decimal(15,2);default:0" json:"tcs_amount"`
	TotalAmount   float64 `gorm:"type:decimal(15,2)" json:"total_amount"`
	
	// References
	InvoiceID  *uuid.UUID `gorm:"type:uuid;index" json:"invoice_id"`
	ShippedDate *time.Time `json:"shipped_date,omitempty"`
	
	// Additional details
	Terms string `gorm:"type:text" json:"terms"`
	Notes string `gorm:"type:text" json:"notes"`
	
	// Relations
	Items   []SalesOrderItem `gorm:"foreignKey:SalesOrderID" json:"items"`
	Invoice *Invoice         `gorm:"foreignKey:InvoiceID" json:"invoice,omitempty"`
	
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// SalesOrderItem represents a line item in a sales order
type SalesOrderItem struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SalesOrderID  uuid.UUID `gorm:"type:uuid;index" json:"sales_order_id"`
	ItemID        uuid.UUID `gorm:"type:uuid;index" json:"item_id"`
	ItemType      string    `gorm:"type:varchar(20);default:'product'" json:"item_type"`
	Name          string    `gorm:"type:varchar(255)" json:"name"`
	Description   string    `gorm:"type:text" json:"description"`
	Quantity      float64   `gorm:"type:decimal(15,2)" json:"quantity"`
	UnitPrice     float64   `gorm:"type:decimal(15,2)" json:"unit_price"`
	Discount      float64   `gorm:"type:decimal(15,2);default:0" json:"discount"`
	Tax           float64   `gorm:"type:decimal(15,2);default:0" json:"tax"`
	Total         float64   `gorm:"type:decimal(15,2)" json:"total"`
	
	// Metadata stores module-specific data
	Metadata json.RawMessage `gorm:"type:jsonb" json:"metadata,omitempty"`
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CanEdit returns true if the sales order can be edited
func (so *SalesOrder) CanEdit() bool {
	return so.Status == SalesOrderStatusDraft
}

// CanConfirm returns true if the sales order can be confirmed
func (so *SalesOrder) CanConfirm() bool {
	return so.Status == SalesOrderStatusDraft
}

// CanCreateInvoice returns true if an invoice can be created from this order
func (so *SalesOrder) CanCreateInvoice() bool {
	return so.Status == SalesOrderStatusConfirmed && so.InvoiceID == nil
}

// CanShip returns true if the order can be marked as shipped
func (so *SalesOrder) CanShip() bool {
	return so.Status == SalesOrderStatusPaid && so.ShippedDate == nil
}

// CanReturn returns true if the order can have returns created
func (so *SalesOrder) CanReturn() bool {
	return (so.Status == SalesOrderStatusPaid || so.Status == SalesOrderStatusShipped) && so.ShippedDate != nil
}

// CanCancel returns true if the order can be cancelled
func (so *SalesOrder) CanCancel() bool {
	return so.Status == SalesOrderStatusDraft || 
	       (so.Status == SalesOrderStatusConfirmed && so.InvoiceID == nil)
}

// CanTransitionTo validates if the order can transition to the new status
func (so *SalesOrder) CanTransitionTo(newStatus SalesOrderStatus) error {
	// Define valid transitions
	validTransitions := map[SalesOrderStatus][]SalesOrderStatus{
		SalesOrderStatusDraft:         {SalesOrderStatusConfirmed, SalesOrderStatusCancelled},
		SalesOrderStatusConfirmed:     {SalesOrderStatusInvoiced, SalesOrderStatusCancelled},
		SalesOrderStatusInvoiced:      {SalesOrderStatusPartiallyPaid, SalesOrderStatusPaid},
		SalesOrderStatusPartiallyPaid: {SalesOrderStatusPaid},
		SalesOrderStatusPaid:          {SalesOrderStatusShipped},
		SalesOrderStatusShipped:       {SalesOrderStatusCompleted},
		SalesOrderStatusCompleted:     {}, // Terminal state
		SalesOrderStatusCancelled:     {}, // Terminal state
	}

	allowedStatuses, exists := validTransitions[so.Status]
	if !exists {
		return fmt.Errorf("unknown current status: %s", so.Status)
	}

	for _, allowed := range allowedStatuses {
		if allowed == newStatus {
			return nil
		}
	}

	return fmt.Errorf("cannot transition from %s to %s", so.Status, newStatus)
}

// CalculateTotals calculates all totals based on line items
func (so *SalesOrder) CalculateTotals() {
	so.SubTotal = 0
	so.DiscountTotal = 0
	so.TaxTotal = 0

	for _, item := range so.Items {
		so.SubTotal += item.Quantity * item.UnitPrice
		so.DiscountTotal += item.Discount
		so.TaxTotal += item.Tax
	}

	// Total = SubTotal - Discount + Tax - TDS + TCS
	so.TotalAmount = so.SubTotal - so.DiscountTotal + so.TaxTotal - so.TDSAmount + so.TCSAmount
}

// Validate performs business rule validation
func (so *SalesOrder) Validate() error {
	if so.OrganizationID == uuid.Nil {
		return fmt.Errorf("organization_id is required")
	}
	if so.CustomerID == uuid.Nil {
		return fmt.Errorf("customer_id is required")
	}
	if len(so.Items) == 0 {
		return fmt.Errorf("at least one item is required")
	}
	if so.TotalAmount < 0 {
		return fmt.Errorf("total amount cannot be negative")
	}
	
	// Validate each item
	for i, item := range so.Items {
		if item.Quantity <= 0 {
			return fmt.Errorf("item %d: quantity must be greater than 0", i+1)
		}
		if item.UnitPrice < 0 {
			return fmt.Errorf("item %d: unit price cannot be negative", i+1)
		}
	}
	
	return nil
}

// TableName specifies the table name for GORM
func (SalesOrder) TableName() string {
	return "sales_orders"
}

// TableName specifies the table name for GORM
func (SalesOrderItem) TableName() string {
	return "sales_order_items"
}
