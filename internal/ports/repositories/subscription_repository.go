package repositories

import (
	"context"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
)

type SubscriptionRepository interface {
	Create(ctx context.Context, sub *domain.Subscription) error
	Update(ctx context.Context, sub *domain.Subscription) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error)
	GetByOrganizationID(ctx context.Context, orgID uuid.UUID) (*domain.Subscription, error)
	GetByRazorpaySubscriptionID(ctx context.Context, razorpaySubID string) (*domain.Subscription, error)
	Delete(ctx context.Context, id uuid.UUID) error

	// Subscription Item Operations
	CreateItem(ctx context.Context, item *domain.SubscriptionItem) error
	UpdateItem(ctx context.Context, item *domain.SubscriptionItem) error
	DeleteItem(ctx context.Context, id uuid.UUID) error

	// Catalog Operations
	GetCatalogItem(ctx context.Context, code string) (*domain.BillingItemCatalog, error)
	ListCatalogItems(ctx context.Context) ([]domain.BillingItemCatalog, error)

	// Subscription Upgrade Operations
	CreateUpgrade(ctx context.Context, upgrade *domain.SubscriptionUpgrade) error
	UpdateUpgrade(ctx context.Context, upgrade *domain.SubscriptionUpgrade) error
	GetUpgradeByOrderID(ctx context.Context, orderID string) (*domain.SubscriptionUpgrade, error)

	// Audit Logs
	CreateAuditLog(ctx context.Context, log *domain.SubscriptionAuditLog) error
	ListAuditLogs(ctx context.Context, subscriptionID uuid.UUID) ([]domain.SubscriptionAuditLog, error)
}
