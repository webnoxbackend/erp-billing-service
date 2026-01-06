package domain

import (
	"time"

	"github.com/google/uuid"
)

// CustomerRM represents a read-optimized version of a Customer
type CustomerRM struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID  uuid.UUID `gorm:"type:uuid;index" json:"organization_id"`
	DisplayName     string    `json:"display_name"`
	CompanyName     string    `json:"company_name"`
	Email           string    `json:"email"`
	Phone           string    `json:"phone"`
	BillingStreet   string    `json:"billing_street"`
	BillingCity     string    `json:"billing_city"`
	BillingState    string    `json:"billing_state"`
	BillingCode     string    `json:"billing_code"`
	BillingCountry  string    `json:"billing_country"`
	ShippingStreet  string    `json:"shipping_street"`
	ShippingCity    string    `json:"shipping_city"`
	ShippingState   string    `json:"shipping_state"`
	ShippingCode    string    `json:"shipping_code"`
	ShippingCountry string    `json:"shipping_country"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// ContactRM represents a read-optimized version of a Contact
type ContactRM struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	CustomerID     uuid.UUID `gorm:"type:uuid;index" json:"customer_id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;index" json:"organization_id"`
	FirstName      string    `json:"first_name"`
	LastName       string    `json:"last_name"`
	Email          string    `json:"email"`
	Phone          string    `json:"phone"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ItemRM represents a read-optimized version of a Service or Part
type ItemRM struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;index" json:"organization_id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	ItemType       string    `json:"item_type"` // "service" or "part"
	Price          float64   `gorm:"type:decimal(15,2)" json:"price"`
	SKU            string    `json:"sku"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// WorkOrderRM represents a read-optimized version of a Work Order
type WorkOrderRM struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	RequestID      *uuid.UUID `gorm:"type:uuid;index" json:"request_id,omitempty"`
	EstimateID     *uuid.UUID `gorm:"type:uuid;index" json:"estimate_id,omitempty"`
	Summary        string     `json:"summary"`
	Priority       string     `json:"priority"`
	Type           string     `json:"type"`
	Status         string     `json:"status"`
	BillingStatus  string     `json:"billing_status"`
	CustomerID     *uuid.UUID `gorm:"type:uuid" json:"customer_id,omitempty"`
	ContactID      *uuid.UUID `gorm:"type:uuid" json:"contact_id,omitempty"`
	ServiceAddress string     `json:"service_address"`
	BillingAddress string     `json:"billing_address"`
	SubTotal       float64    `gorm:"type:decimal(15,2)" json:"sub_total"`
	Discount       float64    `gorm:"type:decimal(15,2)" json:"discount"`
	Adjustment     float64    `gorm:"type:decimal(15,2)" json:"adjustment"`
	GrandTotal     float64    `gorm:"type:decimal(15,2)" json:"grand_total"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// WorkOrderServiceLineRM represents a read-optimized version of a Work Order Service Line
type WorkOrderServiceLineRM struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkOrderID uuid.UUID  `gorm:"type:uuid;index" json:"work_order_id"`
	ServiceID   *uuid.UUID `gorm:"type:uuid" json:"service_id,omitempty"`
	Description string     `json:"description"`
	Quantity    float64    `gorm:"type:decimal(10,2)" json:"quantity"`
	Unit        string     `json:"unit"`
	ListPrice   float64    `gorm:"type:decimal(15,2)" json:"list_price"`
	LineAmount  float64    `gorm:"type:decimal(15,2)" json:"line_amount"`
}

// WorkOrderPartLineRM represents a read-optimized version of a Work Order Part Line
type WorkOrderPartLineRM struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkOrderID uuid.UUID  `gorm:"type:uuid;index" json:"work_order_id"`
	PartID      *uuid.UUID `gorm:"type:uuid" json:"part_id,omitempty"`
	Description string     `json:"description"`
	Quantity    float64    `gorm:"type:decimal(10,2)" json:"quantity"`
	Unit        string     `json:"unit"`
	ListPrice   float64    `gorm:"type:decimal(15,2)" json:"list_price"`
	LineAmount  float64    `gorm:"type:decimal(15,2)" json:"line_amount"`
}
