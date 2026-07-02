package unit

import (
	"context"
	"math"
	"testing"
	"time"

	"erp-billing-service/internal/application"
	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
)

// Mock repos and clients for unit testing
type mockSubscriptionRepo struct {
	subs      map[uuid.UUID]*domain.Subscription
	upgrades  map[string]*domain.SubscriptionUpgrade
	catalog   map[string]*domain.BillingItemCatalog
	auditLogs []domain.SubscriptionAuditLog
}

func newMockSubscriptionRepo() *mockSubscriptionRepo {
	r := &mockSubscriptionRepo{
		subs:     make(map[uuid.UUID]*domain.Subscription),
		upgrades: make(map[string]*domain.SubscriptionUpgrade),
		catalog:  make(map[string]*domain.BillingItemCatalog),
	}
	// Seed mock catalog
	r.catalog["MODULE_EFS"] = &domain.BillingItemCatalog{
		Code:        "MODULE_EFS",
		Name:        "Field Service Management Module",
		Type:        "module",
		BillingType: "per_unit",
		UnitPrice:   500.00,
		TaxRate:     18.00,
		IsActive:    true,
	}
	r.catalog["MODULE_CRM"] = &domain.BillingItemCatalog{
		Code:        "MODULE_CRM",
		Name:        "CRM Module",
		Type:        "module",
		BillingType: "fixed",
		UnitPrice:   2000.00,
		TaxRate:     18.00,
		IsActive:    true,
	}
	r.catalog["MODULE_IMS"] = &domain.BillingItemCatalog{
		Code:        "MODULE_IMS",
		Name:        "Inventory Management System Module",
		Type:        "module",
		BillingType: "fixed",
		UnitPrice:   3000.00,
		TaxRate:     18.00,
		IsActive:    true,
	}
	return r
}

func (m *mockSubscriptionRepo) Create(ctx context.Context, sub *domain.Subscription) error {
	m.subs[sub.OrganizationID] = sub
	return nil
}

func (m *mockSubscriptionRepo) Update(ctx context.Context, sub *domain.Subscription) error {
	m.subs[sub.OrganizationID] = sub
	return nil
}

func (m *mockSubscriptionRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	for _, sub := range m.subs {
		if sub.ID == id {
			return sub, nil
		}
	}
	return nil, nil
}

func (m *mockSubscriptionRepo) GetByOrganizationID(ctx context.Context, orgID uuid.UUID) (*domain.Subscription, error) {
	return m.subs[orgID], nil
}

func (m *mockSubscriptionRepo) GetByRazorpaySubscriptionID(ctx context.Context, razorpaySubID string) (*domain.Subscription, error) {
	for _, sub := range m.subs {
		if sub.RazorpaySubscriptionID != nil && *sub.RazorpaySubscriptionID == razorpaySubID {
			return sub, nil
		}
	}
	return nil, nil
}

func (m *mockSubscriptionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	for orgID, sub := range m.subs {
		if sub.ID == id {
			delete(m.subs, orgID)
			break
		}
	}
	return nil
}

func (m *mockSubscriptionRepo) CreateItem(ctx context.Context, item *domain.SubscriptionItem) error {
	return nil
}

func (m *mockSubscriptionRepo) UpdateItem(ctx context.Context, item *domain.SubscriptionItem) error {
	return nil
}

func (m *mockSubscriptionRepo) DeleteItem(ctx context.Context, id uuid.UUID) error {
	return nil
}

func (m *mockSubscriptionRepo) GetCatalogItem(ctx context.Context, code string) (*domain.BillingItemCatalog, error) {
	return m.catalog[code], nil
}

func (m *mockSubscriptionRepo) ListCatalogItems(ctx context.Context) ([]domain.BillingItemCatalog, error) {
	var list []domain.BillingItemCatalog
	for _, val := range m.catalog {
		list = append(list, *val)
	}
	return list, nil
}

func (m *mockSubscriptionRepo) CreateUpgrade(ctx context.Context, upgrade *domain.SubscriptionUpgrade) error {
	m.upgrades[upgrade.RazorpayOrderID] = upgrade
	return nil
}

func (m *mockSubscriptionRepo) UpdateUpgrade(ctx context.Context, upgrade *domain.SubscriptionUpgrade) error {
	m.upgrades[upgrade.RazorpayOrderID] = upgrade
	return nil
}

func (m *mockSubscriptionRepo) GetUpgradeByOrderID(ctx context.Context, orderID string) (*domain.SubscriptionUpgrade, error) {
	return m.upgrades[orderID], nil
}

func (m *mockSubscriptionRepo) CreateAuditLog(ctx context.Context, log *domain.SubscriptionAuditLog) error {
	m.auditLogs = append(m.auditLogs, *log)
	return nil
}

func (m *mockSubscriptionRepo) ListAuditLogs(ctx context.Context, subscriptionID uuid.UUID) ([]domain.SubscriptionAuditLog, error) {
	return m.auditLogs, nil
}

// Mock Razorpay Client
type mockRazorpayClient struct{}

func (m *mockRazorpayClient) CreatePlan(ctx context.Context, name string, amount float64, currency string) (string, error) {
	return "plan_mock_123", nil
}

func (m *mockRazorpayClient) CreateSubscription(ctx context.Context, planID string, customerEmail string, totalCycles int) (string, string, error) {
	return "sub_mock_abc", "https://checkout.razorpay.com/sub_mock_abc", nil
}

func (m *mockRazorpayClient) UpdateSubscriptionPlan(ctx context.Context, subID string, newPlanID string, scheduleChangeAt string) error {
	return nil
}

func (m *mockRazorpayClient) CreateOrder(ctx context.Context, amount float64, currency string, receipt string) (string, error) {
	return "order_mock_xyz", nil
}

func (m *mockRazorpayClient) VerifyWebhookSignature(body []byte, signature string, secret string) bool {
	return true
}

func TestProratedUpgradeCalculation(t *testing.T) {
	orgID := uuid.New()
	subRepo := newMockSubscriptionRepo()
	razorpay := &mockRazorpayClient{}
	service := application.NewSubscriptionService(subRepo, nil, nil, nil, nil, razorpay)

	// Create initial subscription EFS module (10 units)
	ctx := context.Background()
	_, err := service.CreateSubscription(ctx, dto.CreateSubscriptionRequest{
		OrganizationID: orgID,
		Modules: []dto.SubscriptionModuleInput{
			{ItemCode: "MODULE_EFS", Quantity: 10},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	// Retrieve local subscription and activate it
	sub, _ := subRepo.GetByOrganizationID(ctx, orgID)
	sub.Status = domain.SubscriptionStatusActive
	
	// Set current cycle start to 15 days ago and end to 15 days in the future (30 days total)
	now := time.Now().UTC()
	sub.CurrentPeriodStart = now.Add(-15 * 24 * time.Hour)
	sub.CurrentPeriodEnd = now.Add(15 * 24 * time.Hour)
	sub.RenewalDate = sub.CurrentPeriodEnd
	_ = subRepo.Update(ctx, sub)

	// Perform upgrade: add CRM (fixed: ₹2000) and increase EFS workforce by 5 (₹500 * 5 = ₹2500)
	resp, err := service.UpgradeSubscription(ctx, dto.UpgradeSubscriptionRequest{
		OrganizationID: orgID,
		Modules: []dto.SubscriptionModuleInput{
			{ItemCode: "MODULE_EFS", Quantity: 15}, // Increase from 10 to 15 (diff: 5)
			{ItemCode: "MODULE_CRM", Quantity: 1},  // New module (diff: 1)
		},
	})

	if err != nil {
		t.Fatalf("Failed to upgrade subscription: %v", err)
	}

	// Expected calculations:
	// Base price difference = (5 * 500) + (1 * 2000) = 2500 + 2000 = ₹4500.
	// Prorated days remaining = 15 days out of 30.
	// Prorated amount = 4500 * (15/30) = ₹2250.
	// GST tax amount = 2250 * 0.18 = ₹405.
	// Total amount to charge = ₹2655.
	
	expectedProrated := 2250.00
	expectedTax := 405.00
	expectedTotal := 2655.00

	if math.Abs(resp.ProratedAmount-expectedProrated) > 0.01 {
		t.Errorf("Expected prorated amount %f, got %f", expectedProrated, resp.ProratedAmount)
	}
	if math.Abs(resp.TaxAmount-expectedTax) > 0.01 {
		t.Errorf("Expected tax amount %f, got %f", expectedTax, resp.TaxAmount)
	}
	if math.Abs(resp.TotalAmount-expectedTotal) > 0.01 {
		t.Errorf("Expected total amount %f, got %f", expectedTotal, resp.TotalAmount)
	}
}

func TestScheduledDowngradeTotals(t *testing.T) {
	orgID := uuid.New()
	subRepo := newMockSubscriptionRepo()
	razorpay := &mockRazorpayClient{}
	service := application.NewSubscriptionService(subRepo, nil, nil, nil, nil, razorpay)

	ctx := context.Background()
	_, err := service.CreateSubscription(ctx, dto.CreateSubscriptionRequest{
		OrganizationID: orgID,
		Modules: []dto.SubscriptionModuleInput{
			{ItemCode: "MODULE_EFS", Quantity: 15},
			{ItemCode: "MODULE_CRM", Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("Failed to create subscription: %v", err)
	}

	sub, _ := subRepo.GetByOrganizationID(ctx, orgID)
	sub.Status = domain.SubscriptionStatusActive
	_ = subRepo.Update(ctx, sub)

	// Downgrade: decrease workforce to 10 and remove CRM
	resp, err := service.DowngradeSubscription(ctx, dto.DowngradeSubscriptionRequest{
		OrganizationID: orgID,
		Modules: []dto.SubscriptionModuleInput{
			{ItemCode: "MODULE_EFS", Quantity: 10},
			{ItemCode: "MODULE_CRM", Quantity: 0}, // Removal
		},
	})
	if err != nil {
		t.Fatalf("Failed to downgrade subscription: %v", err)
	}

	// Expected recurring amount calculations:
	// CRM is marked pending_removal (not active in next total, amount = 0)
	// EFS pending quantity is 10.
	// New recurring total = 10 * 500 = ₹5000.
	// New total including tax = 5000 * 1.18 = ₹5900.
	
	expectedRecurring := 5000.00
	expectedTotalRecurring := 5900.00

	if resp.RecurringAmount != expectedRecurring {
		t.Errorf("Expected recurring amount %f, got %f", expectedRecurring, resp.RecurringAmount)
	}
	if resp.TotalRecurringAmount != expectedTotalRecurring {
		t.Errorf("Expected total recurring amount %f, got %f", expectedTotalRecurring, resp.TotalRecurringAmount)
	}
}
