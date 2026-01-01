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
