package domain

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// SubscriptionStatus represents the lifecycle state of a subscription
type SubscriptionStatus string

const (
	SubscriptionStatusCreated   SubscriptionStatus = "created"
	SubscriptionStatusActive    SubscriptionStatus = "active"
	SubscriptionStatusHalted    SubscriptionStatus = "halted"
	SubscriptionStatusCancelled SubscriptionStatus = "cancelled"
)

// Subscription represents the single active subscription of an organization
type Subscription struct {
	ID                     uuid.UUID          `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID         uuid.UUID          `gorm:"type:uuid;uniqueIndex;not null" json:"organization_id"`
	RazorpaySubscriptionID *string            `gorm:"type:varchar(255);uniqueIndex" json:"razorpay_subscription_id"`
	RazorpayPlanID         string             `gorm:"type:varchar(255);not null" json:"razorpay_plan_id"`
	Status                 SubscriptionStatus `gorm:"type:varchar(50);default:'created';not null" json:"status"`
	CurrentPeriodStart     time.Time          `json:"current_period_start"`
	CurrentPeriodEnd       time.Time          `json:"current_period_end"`
	RenewalDate            time.Time          `json:"renewal_date"`
	RecurringAmount        float64            `gorm:"type:decimal(15,2);not null" json:"recurring_amount"` // Monthly base total excl tax
	TaxPercentage          float64            `gorm:"type:decimal(5,2);default:18.00;not null" json:"tax_percentage"`
	TotalRecurringAmount   float64            `gorm:"type:decimal(15,2);not null" json:"total_recurring_amount"` // Monthly total incl tax
	Currency               string             `gorm:"type:varchar(10);default:'INR';not null" json:"currency"`
	CreatedAt              time.Time          `json:"created_at"`
	UpdatedAt              time.Time          `json:"updated_at"`

	Items                  []SubscriptionItem `gorm:"foreignKey:SubscriptionID" json:"items"`
}

// SubscriptionItemStatus represents whether a module is active or scheduled for removal
type SubscriptionItemStatus string

const (
	SubscriptionItemActive         SubscriptionItemStatus = "active"
	SubscriptionItemPendingRemoval SubscriptionItemStatus = "pending_removal"
)

// SubscriptionItem represents individual modules or add-ons included in the subscription
type SubscriptionItem struct {
	ID             uuid.UUID              `gorm:"type:uuid;primaryKey" json:"id"`
	SubscriptionID uuid.UUID              `gorm:"type:uuid;index;not null" json:"subscription_id"`
	ItemCode       string                 `gorm:"type:varchar(100);not null" json:"item_code"` // e.g. MODULE_EFS, MODULE_CRM
	Name           string                 `gorm:"type:varchar(255);not null" json:"name"`
	Type           string                 `gorm:"type:varchar(50);not null" json:"type"`        // "module" or "addon"
	BillingType    string                 `gorm:"type:varchar(50);not null" json:"billing_type"` // "fixed" or "per_unit"
	UnitPrice      float64                `gorm:"type:decimal(15,2);not null" json:"unit_price"`
	Quantity       int                    `gorm:"type:integer;default:1;not null" json:"quantity"`
	Amount         float64                `gorm:"type:decimal(15,2);not null" json:"amount"` // UnitPrice * Quantity
	Status         SubscriptionItemStatus `gorm:"type:varchar(50);default:'active';not null" json:"status"`

	// Downgrade support
	PendingQuantity *int                  `gorm:"type:integer" json:"pending_quantity,omitempty"` // Target count for next cycle

	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// SubscriptionUpgradeStatus represents state of a mid-cycle prorated upgrade
type SubscriptionUpgradeStatus string

const (
	UpgradeStatusPending   SubscriptionUpgradeStatus = "pending"
	UpgradeStatusCompleted SubscriptionUpgradeStatus = "completed"
	UpgradeStatusFailed    SubscriptionUpgradeStatus = "failed"
)

// SubscriptionUpgrade tracks a pending proration payment before updating the subscription
type SubscriptionUpgrade struct {
	ID              uuid.UUID                 `gorm:"type:uuid;primaryKey" json:"id"`
	SubscriptionID  uuid.UUID                 `gorm:"type:uuid;index;not null" json:"subscription_id"`
	RazorpayOrderID string                    `gorm:"type:varchar(255);uniqueIndex;not null" json:"razorpay_order_id"`
	Status          SubscriptionUpgradeStatus `gorm:"type:varchar(50);default:'pending';not null" json:"status"`
	ProratedAmount  float64                   `gorm:"type:decimal(15,2);not null" json:"prorated_amount"` // Net excl GST
	TaxAmount       float64                   `gorm:"type:decimal(15,2);not null" json:"tax_amount"`
	TotalPaid       float64                   `gorm:"type:decimal(15,2);not null" json:"total_paid"` // Net + GST
	UpgradeItems    json.RawMessage           `gorm:"type:jsonb;not null" json:"upgrade_items"`      // DTO list of changes
	CreatedAt       time.Time                 `json:"created_at"`
	UpdatedAt       time.Time                 `json:"updated_at"`
}

// BillingItemCatalog defines the default products and pricing in the system
type BillingItemCatalog struct {
	Code        string    `gorm:"type:varchar(100);primaryKey" json:"code"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Type        string    `gorm:"type:varchar(50);not null" json:"type"`        // "module" or "addon"
	BillingType string    `gorm:"type:varchar(50);not null" json:"billing_type"` // "fixed" or "per_unit"
	UnitPrice   float64   `gorm:"type:decimal(15,2);not null" json:"unit_price"`
	TaxRate     float64   `gorm:"type:decimal(5,2);default:18.00;not null" json:"tax_rate"` // GST %
	IsActive    bool      `gorm:"default:true;not null" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SubscriptionAuditLog records history of plan changes
type SubscriptionAuditLog struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID `gorm:"type:uuid;index;not null" json:"organization_id"`
	SubscriptionID uuid.UUID `gorm:"type:uuid;index;not null" json:"subscription_id"`
	Action         string    `gorm:"type:varchar(100);not null" json:"action"` // "create", "upgrade", "downgrade", "renew", "cancel"
	OldDetails     string    `gorm:"type:text" json:"old_details"`
	NewDetails     string    `gorm:"type:text" json:"new_details"`
	PerformedBy    string    `gorm:"type:varchar(255);not null" json:"performed_by"`
	CreatedAt      time.Time `json:"created_at"`
}

// RecalculateTotals recalculates recurring and total recurring amounts based on items
func (s *Subscription) RecalculateTotals() {
	var total float64
	for _, item := range s.Items {
		if item.Status != SubscriptionItemPendingRemoval {
			// If it's a downgrade, compute based on PendingQuantity if set
			qty := item.Quantity
			if item.PendingQuantity != nil {
				qty = *item.PendingQuantity
			}
			total += item.UnitPrice * float64(qty)
		}
	}
	s.RecurringAmount = total
	taxMultiplier := 1.0 + (s.TaxPercentage / 100.0)
	s.TotalRecurringAmount = total * taxMultiplier
}
