package domain

import (
	"time"
)

type PlanReadOnly struct {
	ID          string    `gorm:"primaryKey;type:uuid" json:"id"`
	Name        string    `gorm:"type:varchar(200);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Price       float64   `gorm:"type:decimal(10,2);default:0" json:"price"`
	ValidDays   int       `gorm:"not null" json:"valid_days"`
	Badge       string    `gorm:"type:varchar(100)" json:"badge"`
	Status      string    `gorm:"type:varchar(20);default:'active'" json:"status"`
	IsPerUser   bool      `gorm:"not null;default:true" json:"is_per_user"`
	IsActive    bool      `gorm:"not null;default:true" json:"is_active"`
	SortOrder   int       `gorm:"not null;default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (PlanReadOnly) TableName() string {
	return "plans_readonly"
}

type PlanRestrictionReadOnly struct {
	ID               string     `gorm:"primaryKey;type:uuid" json:"id"`
	PlanID           string     `gorm:"type:uuid;not null;index:idx_pr_readonly_plan_id" json:"plan_id"`
	RestrictionKey   string     `gorm:"type:varchar(100);not null" json:"restriction_key"`
	RestrictionValue int        `gorm:"not null" json:"restriction_value"`
	Description      string     `gorm:"type:varchar(255)" json:"description"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (PlanRestrictionReadOnly) TableName() string {
	return "plan_restrictions_readonly"
}

type OrganizationSubscriptionReadOnly struct {
	ID                     string     `gorm:"primaryKey;type:uuid" json:"id"`
	OrganizationID         string     `gorm:"type:uuid;not null;index:idx_os_readonly_org_id" json:"organization_id"`
	PlanID                 string     `gorm:"type:uuid;not null" json:"plan_id"`
	Status                 string     `gorm:"type:varchar(20);default:'active'" json:"status"`
	StartDate              time.Time  `gorm:"not null" json:"start_date"`
	ExpiryDate             time.Time  `gorm:"not null" json:"expiry_date"`
	SelectedModuleIDs      string     `gorm:"type:jsonb" json:"selected_module_ids"`
	TotalPrice             float64    `gorm:"type:decimal(10,2);default:0" json:"total_price"`
	UserCount              int        `gorm:"default:1" json:"user_count"`
	Notes                  string     `gorm:"type:text" json:"notes"`
	CancelledAt            *time.Time `gorm:"type:timestamp" json:"cancelled_at,omitempty"`
	TrialEndsAt            *time.Time `gorm:"type:timestamp" json:"trial_ends_at,omitempty"`
	BillingPeriod          string     `gorm:"type:varchar(20);default:'monthly'" json:"billing_period"`
	RazorpaySubscriptionID string     `gorm:"type:varchar(255)" json:"razorpay_subscription_id,omitempty"`
	PaymentMethodID        *string    `gorm:"type:uuid" json:"payment_method_id,omitempty"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

func (OrganizationSubscriptionReadOnly) TableName() string {
	return "organization_subscriptions_readonly"
}

type OrganizationSubscriptionItemReadOnly struct {
	ID             string    `gorm:"primaryKey;type:uuid" json:"id"`
	SubscriptionID string    `gorm:"type:uuid;not null;index:idx_osi_readonly_sub_id" json:"subscription_id"`
	ModuleSlug     string    `gorm:"type:varchar(50);not null" json:"module_slug"`
	PlanID         string    `gorm:"type:uuid;not null" json:"plan_id"`
	Quantity       int       `gorm:"default:1" json:"quantity"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (OrganizationSubscriptionItemReadOnly) TableName() string {
	return "organization_subscription_items_readonly"
}
