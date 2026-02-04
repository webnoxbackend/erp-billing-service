package postgres

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InvoiceRepository struct {
	db *gorm.DB
}

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Create(ctx context.Context, invoice *domain.Invoice) error {
	return r.db.WithContext(ctx).Create(invoice).Error
}

func (r *InvoiceRepository) Update(ctx context.Context, invoice *domain.Invoice) error {
	return r.db.WithContext(ctx).Save(invoice).Error
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	var invoice domain.Invoice
	err := r.db.WithContext(ctx).Preload("Items").Preload("Payments").First(&invoice, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &invoice, nil
}

func (r *InvoiceRepository) List(ctx context.Context, filter map[string]interface{}) ([]domain.Invoice, error) {
	var invoices []domain.Invoice
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("Payments").
		Where(filter).
		Order("created_at desc").
		Find(&invoices).Error
	return invoices, err
}

func (r *InvoiceRepository) ListByModule(ctx context.Context, orgID uuid.UUID, sourceSystem domain.SourceSystem) ([]domain.Invoice, error) {
	var invoices []domain.Invoice
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("Payments").
		Where("organization_id = ? AND source_system = ?", orgID, sourceSystem).
		Order("created_at desc").
		Find(&invoices).Error
	return invoices, err
}

func (r *InvoiceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First delete all invoice items
		if err := tx.Delete(&domain.InvoiceItem{}, "invoice_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete invoice items: %w", err)
		}

		// Then delete all payments
		if err := tx.Delete(&domain.Payment{}, "invoice_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete payments: %w", err)
		}

		// Finally delete the invoice
		if err := tx.Delete(&domain.Invoice{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete invoice: %w", err)
		}

		return nil
	})
}

func (r *InvoiceRepository) GetNextInvoiceNumber(ctx context.Context, orgID uuid.UUID) (string, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&domain.Invoice{}).Where("organization_id = ?", orgID).Count(&count).Error
	if err != nil {
		return "", err
	}

	// Example format: INV-2023-0001
	year := time.Now().Year()
	return fmt.Sprintf("INV-%d-%04d", year, count+1), nil
}

func (r *InvoiceRepository) ClearItems(ctx context.Context, invoiceID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.InvoiceItem{}, "invoice_id = ?", invoiceID).Error
}
