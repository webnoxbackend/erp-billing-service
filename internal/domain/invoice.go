package domain

import (
	"time"

	"github.com/google/uuid"
)

type InvoiceStatus string

const (
	InvoiceStatusDraft   InvoiceStatus = "draft"
	InvoiceStatusSent    InvoiceStatus = "sent"
	InvoiceStatusPartial InvoiceStatus = "partial"
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
	InvoiceNumber   string        `gorm:"type:varchar(50);uniqueIndex" json:"invoice_number"`
	ReferenceNo     string        `gorm:"type:varchar(50)" json:"reference_no"`
	SalesOrder      string        `gorm:"type:varchar(50)" json:"sales_order"`
	PurchaseOrder   string        `gorm:"type:varchar(50)" json:"purchase_order"`
	InvoiceDate     time.Time     `json:"invoice_date"`
	DueDate         time.Time     `json:"due_date"`
	Status          InvoiceStatus `gorm:"type:varchar(20);default:'draft'" json:"status"`
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
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Payment struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;index" json:"organization_id"`
	InvoiceID      uuid.UUID `gorm:"type:uuid;index" json:"invoice_id"`
	Amount         float64   `gorm:"type:decimal(15,2)" json:"amount"`
	PaymentDate    time.Time `json:"payment_date"`
	PaymentMethod  string    `gorm:"type:varchar(50)" json:"payment_method"`
	TransactionRef string    `gorm:"type:varchar(100)" json:"transaction_ref"`
	Notes          string    `gorm:"type:text" json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
