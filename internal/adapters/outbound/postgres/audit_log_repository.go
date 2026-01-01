package postgres

import (
	"context"
	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type auditLogRepository struct {
	db *gorm.DB
}

func NewAuditLogRepository(db *gorm.DB) domain.AuditLogRepository {
	return &auditLogRepository{db: db}
}

func (r *auditLogRepository) Create(ctx context.Context, log *domain.InvoiceAuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *auditLogRepository) ListByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]domain.InvoiceAuditLog, error) {
	var logs []domain.InvoiceAuditLog
	err := r.db.WithContext(ctx).
		Where("invoice_id = ?", invoiceID).
		Order("created_at desc").
		Find(&logs).Error
	return logs, err
}
