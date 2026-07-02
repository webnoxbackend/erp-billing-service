package postgres

import (
	"context"
	"errors"

	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/repositories"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type subscriptionRepository struct {
	db *gorm.DB
}

func NewSubscriptionRepository(db *gorm.DB) repositories.SubscriptionRepository {
	return &subscriptionRepository{db: db}
}

func (r *subscriptionRepository) Create(ctx context.Context, sub *domain.Subscription) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(sub).Error; err != nil {
			return err
		}
		for i := range sub.Items {
			sub.Items[i].SubscriptionID = sub.ID
			if err := tx.Create(&sub.Items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *subscriptionRepository) Update(ctx context.Context, sub *domain.Subscription) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(sub).Error; err != nil {
			return err
		}
		for i := range sub.Items {
			sub.Items[i].SubscriptionID = sub.ID
			if err := tx.Save(&sub.Items[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *subscriptionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Subscription, error) {
	var sub domain.Subscription
	if err := r.db.WithContext(ctx).Preload("Items").First(&sub, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *subscriptionRepository) GetByOrganizationID(ctx context.Context, orgID uuid.UUID) (*domain.Subscription, error) {
	var sub domain.Subscription
	if err := r.db.WithContext(ctx).Preload("Items").First(&sub, "organization_id = ?", orgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *subscriptionRepository) GetByRazorpaySubscriptionID(ctx context.Context, razorpaySubID string) (*domain.Subscription, error) {
	var sub domain.Subscription
	if err := r.db.WithContext(ctx).Preload("Items").First(&sub, "razorpay_subscription_id = ?", razorpaySubID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &sub, nil
}

func (r *subscriptionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&domain.SubscriptionItem{}, "subscription_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&domain.Subscription{}, "id = ?", id).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *subscriptionRepository) CreateItem(ctx context.Context, item *domain.SubscriptionItem) error {
	return r.db.WithContext(ctx).Create(item).Error
}

func (r *subscriptionRepository) UpdateItem(ctx context.Context, item *domain.SubscriptionItem) error {
	return r.db.WithContext(ctx).Save(item).Error
}

func (r *subscriptionRepository) DeleteItem(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.SubscriptionItem{}, "id = ?", id).Error
}

func (r *subscriptionRepository) GetCatalogItem(ctx context.Context, code string) (*domain.BillingItemCatalog, error) {
	var item domain.BillingItemCatalog
	if err := r.db.WithContext(ctx).First(&item, "code = ? AND is_active = true", code).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func (r *subscriptionRepository) ListCatalogItems(ctx context.Context) ([]domain.BillingItemCatalog, error) {
	var items []domain.BillingItemCatalog
	if err := r.db.WithContext(ctx).Find(&items, "is_active = true").Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (r *subscriptionRepository) CreateUpgrade(ctx context.Context, upgrade *domain.SubscriptionUpgrade) error {
	return r.db.WithContext(ctx).Create(upgrade).Error
}

func (r *subscriptionRepository) UpdateUpgrade(ctx context.Context, upgrade *domain.SubscriptionUpgrade) error {
	return r.db.WithContext(ctx).Save(upgrade).Error
}

func (r *subscriptionRepository) GetUpgradeByOrderID(ctx context.Context, orderID string) (*domain.SubscriptionUpgrade, error) {
	var upgrade domain.SubscriptionUpgrade
	if err := r.db.WithContext(ctx).First(&upgrade, "razorpay_order_id = ?", orderID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &upgrade, nil
}

func (r *subscriptionRepository) CreateAuditLog(ctx context.Context, log *domain.SubscriptionAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *subscriptionRepository) ListAuditLogs(ctx context.Context, subscriptionID uuid.UUID) ([]domain.SubscriptionAuditLog, error) {
	var logs []domain.SubscriptionAuditLog
	if err := r.db.WithContext(ctx).Order("created_at desc").Find(&logs, "subscription_id = ?", subscriptionID).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
