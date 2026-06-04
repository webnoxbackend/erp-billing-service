package domain

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CustomerRM represents a read-optimized version of a Customer
type CustomerRM struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID   uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	ExternalKey      *string    `gorm:"size:100" json:"external_key"`
	CustomerType     string     `gorm:"size:20;index" json:"customer_type"`
	DisplayName      string     `gorm:"size:255;index" json:"display_name"`
	CompanyName      string     `gorm:"size:255" json:"company_name"`
	FirstName        string     `gorm:"size:100" json:"first_name"`
	LastName         string     `gorm:"size:100" json:"last_name"`
	Salutation       string     `gorm:"size:20" json:"salutation"`
	Email            string     `gorm:"size:255;index" json:"email"`
	PhoneWork        string     `gorm:"size:50" json:"phone_work"`
	PhoneMobile      string     `gorm:"size:50" json:"phone_mobile"`
	WebsiteURL       string     `gorm:"size:255" json:"website_url"`
	TaxNumber        string     `gorm:"size:50" json:"tax_number"`
	CurrencyCode     string     `gorm:"size:10;default:'INR'" json:"currency_code"`
	PaymentTerms     string     `gorm:"size:50" json:"payment_terms"`
	IsTaxable        bool       `json:"is_taxable"`
	Industry         string     `gorm:"size:100" json:"industry"`
	Rating           string     `gorm:"size:50" json:"rating"`
	Ownership        string     `gorm:"size:50" json:"ownership"`
	AnnualRevenue    float64    `gorm:"type:numeric(18,2)" json:"annual_revenue"`
	PortalEnabled    bool       `json:"portal_enabled"`
	Status           string     `gorm:"size:20;index" json:"status"`
	SourceSystem     string     `gorm:"size:30" json:"source_system"`
	SourceID         string     `gorm:"size:100" json:"source_id"`
	ServiceAddressID *uuid.UUID `gorm:"type:uuid;index" json:"service_address_id"`
	BillingAddressID *uuid.UUID `gorm:"type:uuid;index" json:"billing_address_id"`
	ShippingAddressID *uuid.UUID `gorm:"type:uuid;index" json:"shipping_address_id"`
	AccountOwner     string     `gorm:"size:255" json:"account_owner"`
	AccountSite      string     `gorm:"size:80" json:"account_site"`
	ParentAccountID  *uuid.UUID `gorm:"type:uuid;index" json:"parent_account_id"`
	CustomerLanguage string     `gorm:"size:50;default:'English'" json:"customer_language"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Ignored/removed database columns kept in Go struct for backward compatibility
	Phone           string `gorm:"-" json:"phone"`
	BillingStreet   string `gorm:"-" json:"billing_street"`
	BillingCity     string `gorm:"-" json:"billing_city"`
	BillingState    string `gorm:"-" json:"billing_state"`
	BillingCode     string `gorm:"-" json:"billing_code"`
	BillingCountry  string `gorm:"-" json:"billing_country"`
	ShippingStreet  string `gorm:"-" json:"shipping_street"`
	ShippingCity    string `gorm:"-" json:"shipping_city"`
	ShippingState   string `gorm:"-" json:"shipping_state"`
	ShippingCode    string `gorm:"-" json:"shipping_code"`
	ShippingCountry string `gorm:"-" json:"shipping_country"`
}

func (CustomerRM) TableName() string {
	return "customers_readonly"
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
	Mobile         string    `json:"mobile"`
	IsPrimary      bool      `json:"is_primary"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// JSONB is a custom type for PostgreSQL JSONB fields
type JSONB map[string]interface{}

// Value implements the driver.Valuer interface
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// ItemRM represents a read-optimized version of a Service or Part
// This is a complete read-only replica for sales order and inventory management
type ItemRM struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index:idx_items_org_id" json:"organization_id"`
	SKU            string     `gorm:"type:varchar(255);not null;uniqueIndex:idx_items_org_sku" json:"sku"`
	Name           string     `gorm:"type:varchar(255);not null" json:"name"`
	Description    string     `gorm:"type:text" json:"description"`
	Type           string     `gorm:"type:varchar(20);not null;check:type IN ('goods', 'service');index:idx_items_type" json:"type"`
	Status         string     `gorm:"type:varchar(20);default:'active';index:idx_items_status" json:"status"`
	UnitID         *string    `gorm:"type:varchar(100)" json:"unit_id"`
	Dimensions     JSONB      `gorm:"type:jsonb" json:"dimensions"`
	Weight         JSONB      `gorm:"type:jsonb" json:"weight"`
	ManufacturerID *string    `gorm:"type:varchar(255)" json:"manufacturer_id"`
	BrandID        *string    `gorm:"type:varchar(255)" json:"brand_id"`
	BarCode        string     `gorm:"type:varchar(255)" json:"bar_code"`
	UPC            string     `gorm:"type:varchar(100)" json:"upc"`
	EAN            string     `gorm:"type:varchar(100)" json:"ean"`
	ISBN           string     `gorm:"type:varchar(100)" json:"isbn"`
	MPN            string     `gorm:"type:varchar(100)" json:"mpn"`
	SalesInfo      JSONB      `gorm:"type:jsonb;not null;default:'{\"selling_price\":0,\"selling_currency\":\"INR\",\"sellable\":true,\"taxable\":false,\"discount_allowed\":false}'" json:"sales_info"`
	PurchaseInfo   JSONB      `gorm:"type:jsonb;not null;default:'{\"cost_price\":0,\"cost_currency\":\"INR\",\"purchasable\":true}'" json:"purchase_info"`
	InventoryInfo  JSONB      `gorm:"type:jsonb;not null;default:'{\"track_inventory\":true,\"quantity_on_hand\":0,\"quantity_reserved\":0,\"quantity_available\":0,\"quantity_damaged\":0,\"reorder_level\":0,\"reorder_quantity\":0,\"valuation_method\":\"Weighted Average\"}'" json:"inventory_info"`
	ServiceInfo    JSONB      `gorm:"type:jsonb" json:"service_info"`
	CRMProductInfo JSONB      `gorm:"type:jsonb" json:"crm_product_info"`
	CRMFields      JSONB      `gorm:"type:jsonb" json:"crm_fields"`
	CRMServiceFields JSONB    `gorm:"type:jsonb" json:"crm_service_fields"`
	CreatedBy      *string    `gorm:"type:varchar(255)" json:"created_by"`
	CreatedAt      time.Time  `gorm:"index:idx_items_created_at" json:"created_at"`
	UpdatedBy      *string    `gorm:"type:varchar(255)" json:"updated_by"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `gorm:"index:idx_items_deleted_at" json:"deleted_at,omitempty"`
	CategoryID     *string    `gorm:"type:varchar(255);index:idx_items_category_id" json:"category_id"`
	SyncedAt       time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (ItemRM) TableName() string {
	return "items_readonly"
}

// WorkOrderRM represents a read-optimized version of a Work Order
type WorkOrderRM struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID    uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	RequestID         *uuid.UUID `gorm:"type:uuid;index" json:"request_id,omitempty"`
	EstimateID        *uuid.UUID `gorm:"type:uuid;index" json:"estimate_id,omitempty"`
	ServiceCategoryID *uuid.UUID `gorm:"type:uuid;index" json:"service_category_id,omitempty"`

	Summary           string     `gorm:"size:255;not null" json:"summary"`
	Priority          string     `gorm:"size:50" json:"priority,omitempty"`
	Type              string     `gorm:"size:50" json:"type,omitempty"`
	DueDate           *time.Time `json:"due_date,omitempty"`
	Status            string     `gorm:"size:50;default:'OPEN'" json:"status"`
	BillingStatus     string     `gorm:"size:50;default:'New'" json:"billing_status"`

	AssetID           *uuid.UUID `gorm:"type:uuid;index" json:"asset_id,omitempty"`
	RequiredSkillID   *uuid.UUID `gorm:"type:uuid;index" json:"required_skill_id,omitempty"`

	CustomerID        *uuid.UUID  `gorm:"type:uuid" json:"customer_id,omitempty"`
	Customer          *CustomerRM `gorm:"foreignKey:CustomerID;references:ID" json:"customer,omitempty"`
	ContactID         *uuid.UUID `gorm:"type:uuid" json:"contact_id,omitempty"`
	Email             string     `gorm:"size:255" json:"email,omitempty"`
	Phone             string     `gorm:"size:50" json:"phone,omitempty"`
	Mobile            string     `gorm:"size:50" json:"mobile,omitempty"`

	ServiceAddressID  *uuid.UUID `gorm:"type:uuid;index" json:"service_address_id,omitempty"`
	BillingAddressID  *uuid.UUID `gorm:"type:uuid;index" json:"billing_address_id,omitempty"`

	PreferredDate1    *time.Time `gorm:"type:timestamp" json:"preferred_date1,omitempty"`
	PreferredDate2    *time.Time `gorm:"type:timestamp" json:"preferred_date2,omitempty"`
	PreferredTime     string     `gorm:"size:50" json:"preferred_time,omitempty"`
	PreferenceNote    string     `gorm:"type:text" json:"preference_note,omitempty"`

	SubTotal          float64    `gorm:"type:decimal(15,2);default:0" json:"sub_total"`
	Discount          float64    `gorm:"type:decimal(15,2);default:0" json:"discount"`
	Adjustment        float64    `gorm:"type:decimal(15,2);default:0" json:"adjustment"`
	GrandTotal        float64    `gorm:"type:decimal(15,2);default:0" json:"grand_total"`

	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt          time.Time  `gorm:"autoUpdateTime" json:"synced_at"`

	// Compatibility calculated fields (virtual)
	ServiceAddress    string     `gorm:"-" json:"service_address,omitempty"`
	BillingAddress    string     `gorm:"-" json:"billing_address,omitempty"`

	// Associations (preloaded on detail fetch)
	ServiceLines      []WorkOrderServiceLineRM `gorm:"foreignKey:WorkOrderID" json:"service_lines,omitempty"`
	PartLines         []WorkOrderPartLineRM    `gorm:"foreignKey:WorkOrderID" json:"part_lines,omitempty"`
}

func (WorkOrderRM) TableName() string {
	return "work_orders_readonly"
}

// WorkOrderServiceLineRM represents a read-optimized version of a Work Order Service Line
type WorkOrderServiceLineRM struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkOrderID uuid.UUID  `gorm:"type:uuid;index" json:"work_order_id"`
	ServiceID   *uuid.UUID `gorm:"type:uuid" json:"service_id,omitempty"`
	Description string     `gorm:"type:text" json:"description"`
	Quantity    float64    `gorm:"type:decimal(10,2)" json:"quantity"`
	Unit        string     `gorm:"size:20" json:"unit,omitempty"`
	ListPrice   float64    `gorm:"type:decimal(15,2)" json:"list_price"`
	LineAmount  float64    `gorm:"type:decimal(15,2)" json:"line_amount"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt    time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (WorkOrderServiceLineRM) TableName() string {
	return "work_order_service_lines_readonly"
}

// WorkOrderPartLineRM represents a read-optimized version of a Work Order Part Line
type WorkOrderPartLineRM struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkOrderID uuid.UUID  `gorm:"type:uuid;index" json:"work_order_id"`
	PartID      *uuid.UUID `gorm:"type:uuid" json:"part_id,omitempty"`
	Description string     `gorm:"type:text" json:"description"`
	Quantity    float64    `gorm:"type:decimal(10,2)" json:"quantity"`
	Unit        string     `gorm:"size:20" json:"unit,omitempty"`
	ListPrice   float64    `gorm:"type:decimal(15,2)" json:"list_price"`
	LineAmount  float64    `gorm:"type:decimal(15,2)" json:"line_amount"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt    time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (WorkOrderPartLineRM) TableName() string {
	return "work_order_part_lines_readonly"
}

// OrganizationRM represents a read-optimized version of an Organization
type OrganizationRM struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ProfileID        string    `gorm:"type:varchar(50);not null" json:"profile_id"`
	OrganizationName string    `gorm:"type:varchar(255);not null" json:"organization_name"`
	OrganizationType string    `gorm:"type:varchar(100)" json:"organization_type"`
	Address          string    `gorm:"type:text" json:"address"`
	City             string    `gorm:"type:varchar(100)" json:"city"`
	State            string    `gorm:"type:varchar(100)" json:"state"`
	ZipCode          string    `gorm:"type:varchar(20)" json:"zip_code"`
	Country          string    `gorm:"type:varchar(100)" json:"country"`
	Phone            string    `gorm:"type:varchar(50)" json:"phone"`
	Website          string    `gorm:"type:text" json:"website"`
	Currency         string    `gorm:"type:varchar(10)" json:"currency"`
	Timezone         string    `gorm:"type:varchar(100)" json:"timezone"`
	IsActive         bool      `gorm:"default:true" json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	SyncedAt         time.Time `gorm:"autoUpdateTime" json:"synced_at"`

	// Virtual field — populated at query time from the admin user's profile photo
	IconURL string `gorm:"-" json:"icon_url,omitempty"`
	Email   string `gorm:"-" json:"email,omitempty"`
}

func (OrganizationRM) TableName() string {
	return "organization_readonly"
}

// AddressReadOnly represents address data synced from customer service (read-only)
type AddressReadOnly struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID  uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	CustomerID      *uuid.UUID `gorm:"type:uuid;index" json:"customer_id"`
	ContactID       *uuid.UUID `gorm:"type:uuid;index" json:"contact_id"`
	Attention       string     `gorm:"size:255" json:"attention"`
	Type            string     `gorm:"size:20;index" json:"type"`
	Street1         string     `gorm:"size:255" json:"street1"`
	Street2         string     `gorm:"size:255" json:"street2"`
	City            string     `gorm:"size:100" json:"city"`
	State           string     `gorm:"size:100" json:"state"`
	PostalCode      string     `gorm:"size:20" json:"postal_code"`
	Country         string     `gorm:"size:100" json:"country"`
	Phone           string     `gorm:"size:50" json:"phone"`
	Fax             string     `gorm:"size:50" json:"fax"`
	IsDefault       bool       `json:"is_default"`
	IsPrimary       bool       `json:"is_primary"` // Keep for compatibility
	Territory       string     `gorm:"size:100" json:"territory"`
	Latitude        *float64   `gorm:"type:numeric(15,12)" json:"latitude"`
	Longitude       *float64   `gorm:"type:numeric(15,12)" json:"longitude"`
	NormalizedHash  string     `gorm:"size:255" json:"normalized_hash"`
	GeocodingStatus string     `gorm:"size:50" json:"geocoding_status"`
	Status          string     `gorm:"size:20;index" json:"status"`
	SyncedAt        time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (AddressReadOnly) TableName() string {
	return "addresses_readonly"
}

// ServiceCategoryReadOnly represents service categories synced from serviceandparts service (read-only)
type ServiceCategoryReadOnly struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
	CategoryName   string     `gorm:"type:varchar(255);not null" json:"category_name"`
	CategoryCode   string     `gorm:"type:varchar(100);not null" json:"category_code"`
	Description    string     `gorm:"type:text" json:"description"`
	Type           string     `gorm:"type:varchar(20);not null;default:'service'" json:"type"`
	ImagePath      string     `gorm:"type:text" json:"image_path"`
	Status         string     `gorm:"type:varchar(20);default:'ACTIVE'" json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	SyncedAt       time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
	DeletedAt      *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (ServiceCategoryReadOnly) TableName() string {
	return "service_categories_readonly"
}

// ServiceAppointmentRM represents a read-optimized version of a Service Appointment
type ServiceAppointmentRM struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID       uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	WorkOrderID          uuid.UUID  `gorm:"type:uuid;index" json:"work_order_id"`
	CustomerID           *uuid.UUID `gorm:"type:uuid;index" json:"customer_id,omitempty"`
	AppointmentNumber    string     `gorm:"size:50;index" json:"appointment_number"`
	Subject              string     `gorm:"size:255" json:"subject,omitempty"`
	ScheduledDate        time.Time  `json:"scheduled_date"`
	ScheduledTime        string     `gorm:"size:50" json:"scheduled_time,omitempty"`
	ScheduledStartTime   *time.Time `json:"scheduled_start_time,omitempty"`
	ScheduledEndTime     *time.Time `json:"scheduled_end_time,omitempty"`
	Duration             int        `json:"duration"`
	Status               string     `gorm:"size:50;default:'SCHEDULED'" json:"status"`
	ActualStartTime      *time.Time `json:"actual_start_time,omitempty"`
	ActualEndTime        *time.Time `json:"actual_end_time,omitempty"`
	StartLatitude        *float64   `gorm:"type:decimal(9,6)" json:"start_latitude,omitempty"`
	StartLongitude       *float64   `gorm:"type:decimal(9,6)" json:"start_longitude,omitempty"`
	EndLatitude          *float64   `gorm:"type:decimal(9,6)" json:"end_latitude,omitempty"`
	EndLongitude         *float64   `gorm:"type:decimal(9,6)" json:"end_longitude,omitempty"`
	AssignedTechnicianID *uuid.UUID `gorm:"type:uuid" json:"assigned_technician_id,omitempty"`
	AssignedCrewID       *uuid.UUID `gorm:"type:uuid" json:"assigned_crew_id,omitempty"`
	ServiceAddressID     *uuid.UUID `gorm:"type:uuid;index" json:"service_address_id,omitempty"`
	BillingAddressID     *uuid.UUID `gorm:"type:uuid;index" json:"billing_address_id,omitempty"`
	Notes                string     `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt             time.Time  `gorm:"autoUpdateTime" json:"synced_at"`

	Resources            []ServiceAppointmentResourceRM `gorm:"foreignKey:ServiceAppointmentID" json:"resources,omitempty"`
	TechnicianNames      []string                       `gorm:"-" json:"technician_names,omitempty"`
	Technicians          []UserReadOnly                 `gorm:"-" json:"technicians,omitempty"`
}

func (ServiceAppointmentRM) TableName() string {
	return "service_appointments_readonly"
}

// ServiceAppointmentResourceRM represents a read-optimized version of a Service Appointment Resource
type ServiceAppointmentResourceRM struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID       uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	ServiceAppointmentID uuid.UUID  `gorm:"type:uuid;not null;index" json:"service_appointment_id"`
	ResourceType         string     `gorm:"size:50;not null" json:"resource_type"`
	ResourceID           uuid.UUID  `gorm:"type:uuid;not null;index" json:"resource_id"`
	StartTime            *time.Time `json:"start_time,omitempty"`
	EndTime              *time.Time `json:"end_time,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt             time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (ServiceAppointmentResourceRM) TableName() string {
	return "service_appointment_resources_readonly"
}

// UserReadOnly represents the replicated user data in billing db (read-only)
type UserReadOnly struct {
	ID               string     `gorm:"type:uuid;primaryKey" json:"id"`
	Email            string     `gorm:"type:varchar(255);not null" json:"email"`
	PasswordHash     string     `gorm:"type:varchar(255);not null" json:"-"`
	FirstName        string     `gorm:"type:varchar(100)" json:"first_name"`
	LastName         string     `gorm:"type:varchar(100)" json:"last_name"`
	FullName         string     `gorm:"type:varchar(255)" json:"full_name"`
	Role             string     `gorm:"type:varchar(50);default:'user'" json:"role"`
	OrganizationID   *string    `gorm:"type:varchar(255)" json:"organization_id,omitempty"`
	OrganizationName string     `gorm:"type:varchar(255)" json:"organization_name,omitempty"`
	ProfileID        *string    `gorm:"type:varchar(50)" json:"profile_id,omitempty"`
	EmployeeID       *string    `gorm:"type:varchar(50)" json:"employee_id,omitempty"`
	WorkforceUserID  *string    `gorm:"type:uuid" json:"workforce_user_id,omitempty"`
	CustomerID       *string    `gorm:"type:varchar(50)" json:"customer_id,omitempty"`
	UserType         string     `gorm:"type:varchar(20);default:'regular'" json:"user_type"`
	ProfilePhotoURL  string     `gorm:"type:varchar(255)" json:"profile_photo_url"`
	PhoneNumber      *string    `gorm:"type:varchar(20)" json:"phone_number,omitempty"`
	PinHash          string     `gorm:"type:varchar(255)" json:"-"`
	EmailVerified    *bool      `gorm:"default:false" json:"email_verified"`
	IsActive         *bool      `gorm:"default:false" json:"is_active"`
	InvitationToken  *string    `gorm:"type:varchar(255)" json:"invitation_token,omitempty"`
	InvitationExpiry *time.Time `json:"invitation_expiry,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt         time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (UserReadOnly) TableName() string {
	return "users_readonly"
}

