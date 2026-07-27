package validation

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
		Where("organization_id = ? AND status IN ?", orgID, []string{"active", "trial"}).
		Order("created_at DESC").
		Limit(1).
		Scan(&sub).Error

	if err != nil || sub.ID == "" {
		var expiredCount int64
		c.db.Table("organization_subscriptions_readonly").
			Where("organization_id = ? AND status = ?", orgID, "expired").
			Count(&expiredCount)
		if expiredCount > 0 {
			return false, "Subscription expired. Please renew your plan.", 0, 0, nil
		}
		return false, "No active subscription. Please subscribe to a plan.", 0, 0, nil
	}

	// Check expiry
	if (sub.Status == "active" || sub.Status == "trial") && time.Now().UTC().After(sub.ExpiryDate) {
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

type rawSubscriptionItem struct {
	ID             string    `gorm:"column:id"`
	SubscriptionID string    `gorm:"column:subscription_id"`
	ModuleSlug     string    `gorm:"column:module_slug"`
	PlanID         string    `gorm:"column:plan_id"`
	Quantity       int       `gorm:"column:quantity"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

// StartSync starts the replication process for plans, restrictions, and subscriptions from organization-service DB
func StartSync(ctx context.Context, localDB *gorm.DB, sourceDSN string) {
	if sourceDSN == "" {
		sourceDSN = os.Getenv("ORGANIZATION_DB_URL")
	}
	if sourceDSN == "" {
		sourceDSN = "postgresql://efsorgdbdev:efsorgdbdev@123@192.168.0.26:5455/efsorgdbdev?sslmode=disable"
	}

	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		log.Println("[SubscriptionSync] Initial replication started...")
		syncTables(localDB, sourceDSN)

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				syncTables(localDB, sourceDSN)
			}
		}
	}()
}

func syncTables(localDB *gorm.DB, sourceDSN string) {
	srcDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Printf("[SubscriptionSync] Warning: Failed to connect to source organization-service DB: %v", err)
		return
	}
	sqlDB, _ := srcDB.DB()
	defer sqlDB.Close()

	// Shadow localDB with a silent session for this sync process
	localDB = localDB.Session(&gorm.Session{
		Logger: localDB.Logger.LogMode(logger.Silent),
	})

	// 1. Replicate plans
	var srcPlans []rawPlan
	if err := srcDB.Table("plans").Find(&srcPlans).Error; err == nil {
		for _, p := range srcPlans {
			localDB.Exec(`
				INSERT INTO plans_readonly (id, name, description, price, valid_days, badge, status, is_per_user, is_active, sort_order, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name, description = EXCLUDED.description, price = EXCLUDED.price,
					valid_days = EXCLUDED.valid_days, badge = EXCLUDED.badge, status = EXCLUDED.status,
					is_per_user = EXCLUDED.is_per_user, is_active = EXCLUDED.is_active,
					sort_order = EXCLUDED.sort_order, created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at
			`, p.ID, p.Name, p.Description, p.Price, p.ValidDays, p.Badge, p.Status, p.IsPerUser, p.IsActive, p.SortOrder, p.CreatedAt, p.UpdatedAt)
		}
	}

	// 2. Replicate plan restrictions
	var srcRest []rawRestriction
	if err := srcDB.Table("plan_restrictions").Find(&srcRest).Error; err == nil {
		for _, r := range srcRest {
			localDB.Exec(`
				INSERT INTO plan_restrictions_readonly (id, plan_id, restriction_key, restriction_value, description, created_at, updated_at, deleted_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (id) DO UPDATE SET
					plan_id = EXCLUDED.plan_id, restriction_key = EXCLUDED.restriction_key,
					restriction_value = EXCLUDED.restriction_value, description = EXCLUDED.description,
					created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at, deleted_at = EXCLUDED.deleted_at
			`, r.ID, r.PlanID, r.RestrictionKey, r.RestrictionValue, r.Description, r.CreatedAt, r.UpdatedAt, r.DeletedAt)
		}
	}

	// 3. Replicate organization subscriptions
	var srcSub []rawSubscription
	if err := srcDB.Table("organization_subscriptions").Find(&srcSub).Error; err == nil {
		for _, s := range srcSub {
			localDB.Exec(`
				INSERT INTO organization_subscriptions_readonly (id, organization_id, plan_id, status, start_date, expiry_date, selected_module_ids, total_price, user_count, notes, cancelled_at, trial_ends_at, billing_period, razorpay_subscription_id, payment_method_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (id) DO UPDATE SET
					organization_id = EXCLUDED.organization_id, plan_id = EXCLUDED.plan_id, status = EXCLUDED.status,
					start_date = EXCLUDED.start_date, expiry_date = EXCLUDED.expiry_date, selected_module_ids = EXCLUDED.selected_module_ids,
					total_price = EXCLUDED.total_price, user_count = EXCLUDED.user_count, notes = EXCLUDED.notes,
					cancelled_at = EXCLUDED.cancelled_at, trial_ends_at = EXCLUDED.trial_ends_at, billing_period = EXCLUDED.billing_period,
					razorpay_subscription_id = EXCLUDED.razorpay_subscription_id, payment_method_id = EXCLUDED.payment_method_id,
					created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at
			`, s.ID, s.OrganizationID, s.PlanID, s.Status, s.StartDate, s.ExpiryDate, s.SelectedModuleIDs, s.TotalPrice, s.UserCount, s.Notes, s.CancelledAt, s.TrialEndsAt, s.BillingPeriod, s.RazorpaySubscriptionID, s.PaymentMethodID, s.CreatedAt, s.UpdatedAt)
		}
	}

	// 4. Replicate organization subscription items
	var srcItems []rawSubscriptionItem
	if err := srcDB.Table("organization_subscription_items").Find(&srcItems).Error; err == nil {
		for _, item := range srcItems {
			localDB.Exec(`
				INSERT INTO organization_subscription_items_readonly (id, subscription_id, module_slug, plan_id, quantity, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (id) DO UPDATE SET
					subscription_id = EXCLUDED.subscription_id, module_slug = EXCLUDED.module_slug,
					plan_id = EXCLUDED.plan_id, quantity = EXCLUDED.quantity,
					created_at = EXCLUDED.created_at, updated_at = EXCLUDED.updated_at
			`, item.ID, item.SubscriptionID, item.ModuleSlug, item.PlanID, item.Quantity, item.CreatedAt, item.UpdatedAt)
		}
	}
}
