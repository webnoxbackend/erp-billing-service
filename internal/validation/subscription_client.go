package validation

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// RestrictionKeys for use across billing service
const (
	RestrictionKeyMaxInvoices       = "max_invoices"
	RestrictionKeyMaxSalesOrders    = "max_sales_orders"
	RestrictionKeyMaxSalesReturns   = "max_sales_returns"
	RestrictionKeyMaxBills          = "max_bills"
	RestrictionKeyMaxVendorCredits = "max_vendor_credits"
	RestrictionKeyMaxPayments       = "max_payments"
)

// imsRestrictionKeys is the set of restriction keys that belong to IMS/Inventory module.
var imsRestrictionKeys = map[string]bool{
	RestrictionKeyMaxInvoices:       true,
	RestrictionKeyMaxSalesOrders:    true,
	RestrictionKeyMaxSalesReturns:   true,
	RestrictionKeyMaxBills:          true,
	RestrictionKeyMaxVendorCredits:  true,
	RestrictionKeyMaxPayments:       true,
}

// SubscriptionClient is a local validation client that queries local *_readonly replica tables
type SubscriptionClient struct {
	db *gorm.DB
}

var globalDB *gorm.DB

// InitDB initializes the global GORM db for subscription validations
func InitDB(db *gorm.DB) {
	globalDB = db
}

// NewSubscriptionClient creates a new SubscriptionClient
func NewSubscriptionClient() *SubscriptionClient {
	return &SubscriptionClient{
		db: globalDB,
	}
}

// ValidateRestriction checks if the org can create a new resource locally
func (c *SubscriptionClient) ValidateRestriction(orgID, restrictionKey string) (bool, string, error) {
	allowed, msg, _, _, err := c.ValidateRestrictionDetailed(orgID, restrictionKey)
	return allowed, msg, err
}

// ValidateRestrictionDetailed checks restriction with full details
// Returns (allowed, message, limit, currentValue, error)
func (c *SubscriptionClient) ValidateRestrictionDetailed(orgID, restrictionKey string) (bool, string, int, int, error) {
	if c.db == nil {
		return true, "no db connection — allowed (fail-open)", -1, 0, nil
	}

	// 1. Get active subscription from organization_subscriptions_readonly
	var sub struct {
		ID         string
		PlanID     string
		Status     string
		ExpiryDate time.Time
	}
	err := c.db.Table("organization_subscriptions_readonly").
		Where("organization_id = ? AND LOWER(status) IN ?", orgID, []string{"active", "trial", "paid"}).
		Order("created_at DESC").
		Limit(1).
		Scan(&sub).Error

	if err != nil || sub.ID == "" {
		var expiredCount int64
		c.db.Table("organization_subscriptions_readonly").
			Where("organization_id = ? AND LOWER(status) = ?", orgID, "expired").
			Count(&expiredCount)
		if expiredCount > 0 {
			return false, "Subscription expired. Please renew your plan.", 0, 0, nil
		}

		// Fail-open: if replica table has 0 rows, Kafka sync is pending
		var totalSubRows int64
		c.db.Table("organization_subscriptions_readonly").Count(&totalSubRows)
		if totalSubRows == 0 {
			log.Printf("[SubscriptionClient] Warning: organization_subscriptions_readonly has 0 total rows (Kafka sync pending) — allowing %s for org %s (fail-open)", restrictionKey, orgID)
			return true, "Subscription sync pending — allowed (fail-open)", -1, 0, nil
		}

		// Fail-open: org not found in readonly table but table has rows — newly registered org,
		// Kafka OrganizationSubscribed event may not have been consumed yet.
		var orgAnySubCount int64
		c.db.Table("organization_subscriptions_readonly").
			Where("organization_id = ?", orgID).
			Count(&orgAnySubCount)
		if orgAnySubCount == 0 {
			log.Printf("[SubscriptionClient] Warning: No subscription for org %s in billing readonly table — newly registered org (Kafka event pending). Allowing %s (fail-open)", orgID, restrictionKey)
			return true, "New organization — subscription sync pending, allowed (fail-open)", -1, 0, nil
		}

		return false, "No active subscription. Please subscribe to a plan.", 0, 0, nil
	}

	// Check expiry
	statusLower := strings.ToLower(sub.Status)
	if (statusLower == "active" || statusLower == "trial" || statusLower == "paid") && time.Now().UTC().After(sub.ExpiryDate) {
		return false, "Subscription expired. Please renew your plan to perform this action.", 0, 0, nil
	}

	// Active trial mode has no restrictions
	if sub.Status == "trial" && sub.ExpiryDate.After(time.Now().UTC()) {
		return true, "Active trial mode — unlimited access", -1, 0, nil
	}

	// 2. Collect all subscribed plan_ids (from main sub and modular subscription items like crm, ims, efs)
	var planIDs []string
	if sub.PlanID != "" {
		planIDs = append(planIDs, sub.PlanID)
	}

	var itemPlanIDs []string
	c.db.Table("organization_subscription_items_readonly").
		Where("subscription_id = ?", sub.ID).
		Pluck("plan_id", &itemPlanIDs)

	for _, pid := range itemPlanIDs {
		if pid != "" {
			planIDs = append(planIDs, pid)
		}
	}

	if len(planIDs) == 0 {
		return true, "No plan IDs found — allowed (fail-open)", -1, 0, nil
	}

	// 3. Query restriction value from plan_restrictions_readonly across all subscribed plans
	var restriction struct {
		RestrictionValue int
	}
	var count int64
	c.db.Table("plan_restrictions_readonly").
		Where("plan_id IN ? AND restriction_key = ? AND deleted_at IS NULL", planIDs, restrictionKey).
		Count(&count)
	if count == 0 {
		return true, "No limit specified", -1, 0, nil
	}

	err = c.db.Table("plan_restrictions_readonly").
		Where("plan_id IN ? AND restriction_key = ? AND deleted_at IS NULL", planIDs, restrictionKey).
		Order("restriction_value ASC").
		Scan(&restriction).Error
	if err != nil {
		return true, "No limit specified", -1, 0, nil
	}

	limitValue := restriction.RestrictionValue

	if strings.HasPrefix(restrictionKey, "allow_") {
		allowed := limitValue == 1
		msg := "Feature allowed"
		if !allowed {
			msg = "Feature disabled under your current plan"
		}
		return allowed, msg, limitValue, 0, nil
	}

	if limitValue == -1 {
		return true, "Unlimited usage allowed", -1, 0, nil
	}

	// 4. Calculate dynamic local usage count
	currentValue := 0
	switch restrictionKey {
	case RestrictionKeyMaxInvoices:
		var cnt int64
		c.db.Table("invoices").Where("organization_id = ? AND deleted_at IS NULL", orgID).Count(&cnt)
		currentValue = int(cnt)

	case RestrictionKeyMaxSalesOrders:
		var cnt int64
		c.db.Table("sales_orders").Where("organization_id = ? AND deleted_at IS NULL", orgID).Count(&cnt)
		currentValue = int(cnt)

	case RestrictionKeyMaxSalesReturns:
		var cnt int64
		c.db.Table("sales_returns").Where("organization_id = ? AND deleted_at IS NULL", orgID).Count(&cnt)
		currentValue = int(cnt)
	}

	allowed := currentValue < limitValue
	msg := fmt.Sprintf("Usage: %d / %d", currentValue, limitValue)
	if !allowed {
		switch restrictionKey {
		case RestrictionKeyMaxSalesOrders:
			msg = fmt.Sprintf("Sales order creation limit reached (%d/%d) under your purchased IMS plan. Please upgrade your subscription to create more sales orders.", currentValue, limitValue)
		case RestrictionKeyMaxSalesReturns:
			msg = fmt.Sprintf("Sales return creation limit reached (%d/%d) under your purchased IMS plan. Please upgrade your subscription to create more sales returns.", currentValue, limitValue)
		default:
			msg = fmt.Sprintf("(%d/%d). Please upgrade your subscription.", currentValue, limitValue)
		}
	}

	return allowed, msg, limitValue, currentValue, nil
}

// IncrementUsage is a local no-op since usage is calculated dynamically via DB COUNT
func (c *SubscriptionClient) IncrementUsage(orgID, restrictionKey string) error {
	return nil
}

// DecrementUsage is a local no-op since usage is calculated dynamically via DB COUNT
func (c *SubscriptionClient) DecrementUsage(orgID, restrictionKey string) error {
	return nil
}

// =========================================================================
// Real-time Database Replication Worker
// =========================================================================

type rawPlan struct {
	ID          string    `gorm:"column:id"`
	Name        string    `gorm:"column:name"`
	Description string    `gorm:"column:description"`
	Price       float64   `gorm:"column:price"`
	ValidDays   int       `gorm:"column:valid_days"`
	Badge       string    `gorm:"column:badge"`
	Status      string    `gorm:"column:status"`
	IsPerUser   bool      `gorm:"column:is_per_user"`
	IsActive    bool      `gorm:"column:is_active"`
	SortOrder   int       `gorm:"column:sort_order"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

type rawRestriction struct {
	ID               string     `gorm:"column:id"`
	PlanID           string     `gorm:"column:plan_id"`
	RestrictionKey   string     `gorm:"column:restriction_key"`
	RestrictionValue int        `gorm:"column:restriction_value"`
	Description      string     `gorm:"column:description"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
	DeletedAt        *time.Time `gorm:"column:deleted_at"`
}

type rawSubscription struct {
	ID                     string     `gorm:"column:id"`
	OrganizationID         string     `gorm:"column:organization_id"`
	PlanID                 string     `gorm:"column:plan_id"`
	Status                 string     `gorm:"column:status"`
	StartDate              time.Time  `gorm:"column:start_date"`
	ExpiryDate             time.Time  `gorm:"column:expiry_date"`
	SelectedModuleIDs      string     `gorm:"column:selected_module_ids"`
	TotalPrice             float64    `gorm:"column:total_price"`
	UserCount              int        `gorm:"column:user_count"`
	Notes                  string     `gorm:"column:notes"`
	CancelledAt            *time.Time `gorm:"column:cancelled_at"`
	TrialEndsAt            *time.Time `gorm:"column:trial_ends_at"`
	BillingPeriod          string     `gorm:"column:billing_period"`
	RazorpaySubscriptionID string     `gorm:"column:razorpay_subscription_id"`
	PaymentMethodID        *string    `gorm:"column:payment_method_id"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	UpdatedAt              time.Time  `gorm:"column:updated_at"`
}

// StartSync is retained for backward compatibility but is now a no-op.
// Billing subscription data is synced via Kafka (subscription_consumer.go) instead of direct DB polling.
func StartSync(ctx context.Context, localDB *gorm.DB, sourceDSN string) {
	log.Println("[BillingSubscriptionSync] Direct DB sync is deprecated. Using Kafka-based sync instead.")
}


