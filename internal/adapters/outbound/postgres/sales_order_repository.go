package postgres

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SalesOrderRepository implements the sales order repository interface
type SalesOrderRepository struct {
	db *gorm.DB
}

// NewSalesOrderRepository creates a new sales order repository
func NewSalesOrderRepository(db *gorm.DB) *SalesOrderRepository {
	return &SalesOrderRepository{db: db}
}

// Create creates a new sales order
func (r *SalesOrderRepository) Create(order *domain.SalesOrder) error {
	return r.db.Create(order).Error
}

// Update updates an existing sales order
func (r *SalesOrderRepository) Update(order *domain.SalesOrder) error {
	return r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(order).Error
}

// FindByID retrieves a sales order by ID with all relations
func (r *SalesOrderRepository) FindByID(id uuid.UUID) (*domain.SalesOrder, error) {
	var order domain.SalesOrder
	err := r.db.Preload("Items").Preload("Invoice").First(&order, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// FindByOrderNumber retrieves a sales order by order number
func (r *SalesOrderRepository) FindByOrderNumber(orderNumber string) (*domain.SalesOrder, error) {
	var order domain.SalesOrder
	err := r.db.Preload("Items").First(&order, "order_number = ?", orderNumber).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// List retrieves sales orders with filters and pagination
func (r *SalesOrderRepository) List(orgID uuid.UUID, filters *dto.SalesOrderFilters) ([]*domain.SalesOrder, int64, error) {
	var orders []*domain.SalesOrder
	var total int64

	query := r.db.Where("organization_id = ?", orgID)

	// Apply filters
	if filters.Status != nil {
		query = query.Where("status = ?", *filters.Status)
	}
	if filters.CustomerID != nil {
		query = query.Where("customer_id = ?", *filters.CustomerID)
	}
	if filters.FromDate != nil {
		query = query.Where("order_date >= ?", *filters.FromDate)
	}
	if filters.ToDate != nil {
		query = query.Where("order_date <= ?", *filters.ToDate)
	}
	if filters.Search != nil {
		query = query.Where("order_number LIKE ?", "%"+*filters.Search+"%")
	}

	// Count total
	if err := query.Model(&domain.SalesOrder{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (filters.Page - 1) * filters.PageSize
	query = query.Offset(offset).Limit(filters.PageSize)

	// Fetch orders
	err := query.Preload("Items").Order("created_at desc").Find(&orders).Error
	return orders, total, err
}

// Delete soft deletes a sales order
func (r *SalesOrderRepository) Delete(id uuid.UUID) error {
	return r.db.Delete(&domain.SalesOrder{}, "id = ?", id).Error
}

// GenerateOrderNumber generates a unique order number for the organization
func (r *SalesOrderRepository) GenerateOrderNumber(orgID uuid.UUID) (string, error) {
	var count int64
	today := time.Now().Format("20060102")
	
	// Count orders created today for this organization
	err := r.db.Model(&domain.SalesOrder{}).
		Where("organization_id = ? AND order_number LIKE ?", orgID, "SO-"+today+"%").
		Count(&count).Error
	if err != nil {
		return "", err
	}

	// Format: SO-YYYYMMDD-XXXX
	return fmt.Sprintf("SO-%s-%04d", today, count+1), nil
}

// FindByInvoiceID retrieves a sales order by invoice ID
func (r *SalesOrderRepository) FindByInvoiceID(invoiceID uuid.UUID) (*domain.SalesOrder, error) {
	var order domain.SalesOrder
	err := r.db.Preload("Items").First(&order, "invoice_id = ?", invoiceID).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// UpdateStatus updates the status of a sales order
func (r *SalesOrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SalesOrderStatus) error {
	return r.db.WithContext(ctx).Model(&domain.SalesOrder{}).Where("id = ?", id).Update("status", status).Error
}

