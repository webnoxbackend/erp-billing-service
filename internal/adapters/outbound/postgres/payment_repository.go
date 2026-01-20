package postgres

import (
	"context"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PaymentRepository struct {
	db *gorm.DB
}

func NewPaymentRepository(db *gorm.DB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	return r.db.WithContext(ctx).Create(payment).Error
}

func (r *PaymentRepository) GetByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]domain.Payment, error) {
	var payments []domain.Payment
	err := r.db.WithContext(ctx).Where("invoice_id = ?", invoiceID).Find(&payments).Error
	return payments, err
}

func (r *PaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	var payment domain.Payment
	err := r.db.WithContext(ctx).First(&payment, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *PaymentRepository) Update(ctx context.Context, payment *domain.Payment) error {
	return r.db.WithContext(ctx).Save(payment).Error
}

func (r *PaymentRepository) ListByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]domain.Payment, error) {
	var payments []domain.Payment
	err := r.db.WithContext(ctx).Where("invoice_id = ?", invoiceID).Order("payment_date desc").Find(&payments).Error
	return payments, err
}

func (r *PaymentRepository) List(ctx context.Context) ([]domain.Payment, error) {
	var payments []domain.Payment
	err := r.db.WithContext(ctx).Order("payment_date desc").Find(&payments).Error
	return payments, err
}
