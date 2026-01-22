package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SalesReturnStatus represents the status of a sales return
type SalesReturnStatus string

const (
	SalesReturnStatusDraft    SalesReturnStatus = "draft"
	SalesReturnStatusApproved SalesReturnStatus = "approved"
	SalesReturnStatusReceived SalesReturnStatus = "received"
	SalesReturnStatusRefunded SalesReturnStatus = "refunded"
)

// SalesReturn represents a return for a sales order
type SalesReturn struct {
	ID             uuid.UUID         `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID         `gorm:"type:uuid;index" json:"organization_id"`
	SalesOrderID   uuid.UUID         `gorm:"type:uuid;index" json:"sales_order_id"`
	ReturnNumber   *string           `gorm:"type:varchar(50);unique" json:"return_number"`
	ReturnDate     time.Time         `json:"return_date"`
	Status         SalesReturnStatus `gorm:"type:varchar(20);default:'draft';index" json:"status"`
	
	// Financial
	ReturnAmount float64 `gorm:"type:decimal(15,2)" json:"return_amount"`
	
	// Workflow dates
	ApprovedDate *time.Time `json:"approved_date,omitempty"`
	ReceivedDate *time.Time `json:"received_date,omitempty"`
	RefundedDate *time.Time `json:"refunded_date,omitempty"`
	
	// Details
	ReturnReason   string  `gorm:"type:text" json:"return_reason"`
	Notes          string  `gorm:"type:text" json:"notes"`
	ReceivingNotes string  `gorm:"type:text" json:"receiving_notes"`
	
	// References
	RefundPaymentID *uuid.UUID `gorm:"type:uuid;index" json:"refund_payment_id"`
	
	// Relations
	Items        []SalesReturnItem `gorm:"foreignKey:SalesReturnID" json:"items"`
	SalesOrder   *SalesOrder       `gorm:"foreignKey:SalesOrderID" json:"sales_order,omitempty"`
	RefundPayment *Payment         `gorm:"foreignKey:RefundPaymentID" json:"refund_payment,omitempty"`
	
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// SalesReturnItem represents a line item in a sales return
type SalesReturnItem struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	SalesReturnID     uuid.UUID `gorm:"type:uuid;index" json:"sales_return_id"`
	SalesOrderItemID  uuid.UUID `gorm:"type:uuid;index" json:"sales_order_item_id"`
	ReturnedQuantity  float64   `gorm:"type:decimal(15,2)" json:"returned_quantity"`
	UnitPrice         float64   `gorm:"type:decimal(15,2)" json:"unit_price"`
	Tax               float64   `gorm:"type:decimal(15,2);default:0" json:"tax"`
	Total             float64   `gorm:"type:decimal(15,2)" json:"total"`
	Reason            string    `gorm:"type:text" json:"reason"`
	
	// Relations
	SalesOrderItem *SalesOrderItem `gorm:"foreignKey:SalesOrderItemID" json:"sales_order_item,omitempty"`
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CanApprove returns true if the return can be approved
func (sr *SalesReturn) CanApprove() bool {
	return sr.Status == SalesReturnStatusDraft
}

// CanReceive returns true if the return can be marked as received
func (sr *SalesReturn) CanReceive() bool {
	return sr.Status == SalesReturnStatusApproved
}

// CanRefund returns true if the return can be refunded
func (sr *SalesReturn) CanRefund() bool {
	return sr.Status == SalesReturnStatusReceived && sr.RefundPaymentID == nil
}

// CanEdit returns true if the return can be edited
func (sr *SalesReturn) CanEdit() bool {
	return sr.Status == SalesReturnStatusDraft || sr.Status == SalesReturnStatusApproved
}

// CanTransitionTo validates if the return can transition to the new status
func (sr *SalesReturn) CanTransitionTo(newStatus SalesReturnStatus) error {
	// Define valid transitions
	validTransitions := map[SalesReturnStatus][]SalesReturnStatus{
		SalesReturnStatusDraft:    {SalesReturnStatusApproved},
		SalesReturnStatusApproved: {SalesReturnStatusReceived},
		SalesReturnStatusReceived: {SalesReturnStatusRefunded},
		SalesReturnStatusRefunded: {}, // Terminal state
	}

	allowedStatuses, exists := validTransitions[sr.Status]
	if !exists {
		return fmt.Errorf("unknown current status: %s", sr.Status)
	}

	for _, allowed := range allowedStatuses {
		if allowed == newStatus {
			return nil
		}
	}

	return fmt.Errorf("cannot transition from %s to %s", sr.Status, newStatus)
}

// CalculateTotals calculates the total return amount based on items
func (sr *SalesReturn) CalculateTotals() {
	sr.ReturnAmount = 0
	for _, item := range sr.Items {
		sr.ReturnAmount += item.Total
	}
}

// ValidateReturnQuantity validates that return quantities don't exceed original order quantities
func (sr *SalesReturn) ValidateReturnQuantity(originalItems []SalesOrderItem) error {
	// Create a map of original quantities
	originalQty := make(map[uuid.UUID]float64)
	for _, item := range originalItems {
		originalQty[item.ID] = item.Quantity
	}
	
	// Validate each return item
	for i, returnItem := range sr.Items {
		originalQuantity, exists := originalQty[returnItem.SalesOrderItemID]
		if !exists {
			return fmt.Errorf("item %d: sales order item not found", i+1)
		}
		
		if returnItem.ReturnedQuantity <= 0 {
			return fmt.Errorf("item %d: returned quantity must be greater than 0", i+1)
		}
		
		if returnItem.ReturnedQuantity > originalQuantity {
			return fmt.Errorf("item %d: returned quantity (%.2f) exceeds original quantity (%.2f)", 
				i+1, returnItem.ReturnedQuantity, originalQuantity)
		}
	}
	
	return nil
}

// Validate performs business rule validation
func (sr *SalesReturn) Validate() error {
	if sr.OrganizationID == uuid.Nil {
		return fmt.Errorf("organization_id is required")
	}
	if sr.SalesOrderID == uuid.Nil {
		return fmt.Errorf("sales_order_id is required")
	}
	if len(sr.Items) == 0 {
		return fmt.Errorf("at least one item is required")
	}
	if sr.ReturnAmount < 0 {
		return fmt.Errorf("return amount cannot be negative")
	}
	if sr.ReturnReason == "" {
		return fmt.Errorf("return reason is required")
	}
	
	return nil
}

// TableName specifies the table name for GORM
func (SalesReturn) TableName() string {
	return "sales_returns"
}

// TableName specifies the table name for GORM
func (SalesReturnItem) TableName() string {
	return "sales_return_items"
}
