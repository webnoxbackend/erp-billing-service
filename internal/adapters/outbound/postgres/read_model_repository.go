package postgres

import (
	"context"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ReadModelRepository struct {
	db *gorm.DB
}

func NewReadModelRepository(db *gorm.DB) *ReadModelRepository {
	return &ReadModelRepository{db: db}
}

func (r *ReadModelRepository) GetCustomer(ctx context.Context, id uuid.UUID) (*domain.CustomerRM, error) {
	var rm domain.CustomerRM
	err := r.db.WithContext(ctx).First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rm, nil
}

func (r *ReadModelRepository) SearchCustomers(ctx context.Context, orgID uuid.UUID, query string) ([]domain.CustomerRM, error) {
	var res []domain.CustomerRM
	q := "%" + query + "%"
	err := r.db.WithContext(ctx).Where("organization_id = ? AND (display_name ILIKE ? OR company_name ILIKE ?)", orgID, q, q).Limit(20).Find(&res).Error
	return res, err
}

func (r *ReadModelRepository) GetItem(ctx context.Context, id uuid.UUID) (*domain.ItemRM, error) {
	var rm domain.ItemRM
	err := r.db.WithContext(ctx).First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rm, nil
}

func (r *ReadModelRepository) SearchItems(ctx context.Context, orgID uuid.UUID, query string) ([]domain.ItemRM, error) {
	var res []domain.ItemRM
	q := "%" + query + "%"
	err := r.db.WithContext(ctx).Where("organization_id = ? AND (name ILIKE ? OR sku ILIKE ?)", orgID, q, q).Limit(20).Find(&res).Error
	return res, err
}

func (r *ReadModelRepository) GetContact(ctx context.Context, id uuid.UUID) (*domain.ContactRM, error) {
	var rm domain.ContactRM
	err := r.db.WithContext(ctx).First(&rm, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &rm, nil
}

func (r *ReadModelRepository) SearchContacts(ctx context.Context, orgID uuid.UUID, customerID uuid.UUID, query string) ([]domain.ContactRM, error) {
	var res []domain.ContactRM
	q := "%" + query + "%"
	db := r.db.WithContext(ctx).Where("organization_id = ?", orgID)
	if customerID != uuid.Nil {
		db = db.Where("customer_id = ?", customerID)
	}
	err := db.Where("(first_name ILIKE ? OR last_name ILIKE ? OR email ILIKE ?)", q, q, q).Limit(20).Find(&res).Error
	return res, err
}
