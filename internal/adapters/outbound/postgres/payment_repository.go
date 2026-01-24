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

// ListByModule returns payments filtered by invoice source_system
func (r *PaymentRepository) ListByModule(ctx context.Context, orgID uuid.UUID, sourceSystem domain.SourceSystem) ([]domain.Payment, error) {
	var payments []domain.Payment
	
	// Join with invoices table to filter by source_system
	err := r.db.WithContext(ctx).
		Joins("JOIN invoices ON invoices.id = payments.invoice_id").
		Where("payments.organization_id = ? AND invoices.source_system = ?", orgID, sourceSystem).
		Order("payments.payment_date desc").
		Find(&payments).Error
	
	return payments, err
}

