package postgres

import (
	"fmt"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SalesReturnRepository implements the sales return repository interface
type SalesReturnRepository struct {
	db *gorm.DB
}

// NewSalesReturnRepository creates a new sales return repository
func NewSalesReturnRepository(db *gorm.DB) *SalesReturnRepository {
	return &SalesReturnRepository{db: db}
}

// Create creates a new sales return
func (r *SalesReturnRepository) Create(salesReturn *domain.SalesReturn) error {
	return r.db.Create(salesReturn).Error
}

// Update updates an existing sales return
func (r *SalesReturnRepository) Update(salesReturn *domain.SalesReturn) error {
	return r.db.Session(&gorm.Session{FullSaveAssociations: true}).Save(salesReturn).Error
}

// FindByID retrieves a sales return by ID with all relations
func (r *SalesReturnRepository) FindByID(id uuid.UUID) (*domain.SalesReturn, error) {
	var salesReturn domain.SalesReturn
	err := r.db.Preload("Items").Preload("Items.SalesOrderItem").
		Preload("SalesOrder").Preload("RefundPayment").
		First(&salesReturn, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &salesReturn, nil
}

// FindByReturnNumber retrieves a sales return by return number
func (r *SalesReturnRepository) FindByReturnNumber(returnNumber string) (*domain.SalesReturn, error) {
	var salesReturn domain.SalesReturn
	err := r.db.Preload("Items").First(&salesReturn, "return_number = ?", returnNumber).Error
	if err != nil {
		return nil, err
	}
	return &salesReturn, nil
}

// FindBySalesOrderID retrieves all sales returns for a sales order
func (r *SalesReturnRepository) FindBySalesOrderID(orderID uuid.UUID) ([]*domain.SalesReturn, error) {
	var salesReturns []*domain.SalesReturn
	err := r.db.Preload("Items").Where("sales_order_id = ?", orderID).
		Order("created_at desc").Find(&salesReturns).Error
	return salesReturns, err
}

// List retrieves sales returns with filters and pagination
func (r *SalesReturnRepository) List(orgID uuid.UUID, filters *dto.SalesReturnFilters) ([]*domain.SalesReturn, int64, error) {
	var salesReturns []*domain.SalesReturn
	var total int64

	query := r.db.Where("organization_id = ?", orgID)

	// Apply filters
	if filters.Status != nil {
		query = query.Where("status = ?", *filters.Status)
	}
	if filters.SalesOrderID != nil {
		query = query.Where("sales_order_id = ?", *filters.SalesOrderID)
	}
	if filters.FromDate != nil {
		query = query.Where("return_date >= ?", *filters.FromDate)
	}
	if filters.ToDate != nil {
		query = query.Where("return_date <= ?", *filters.ToDate)
	}
	if filters.Search != nil {
		query = query.Where("return_number LIKE ?", "%"+*filters.Search+"%")
	}

	// Count total
	if err := query.Model(&domain.SalesReturn{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	offset := (filters.Page - 1) * filters.PageSize
	query = query.Offset(offset).Limit(filters.PageSize)

	// Fetch returns
	err := query.Preload("Items").Order("created_at desc").Find(&salesReturns).Error
	return salesReturns, total, err
}

// GenerateReturnNumber generates a unique return number for the organization
func (r *SalesReturnRepository) GenerateReturnNumber(orgID uuid.UUID) (string, error) {
	var count int64
	today := time.Now().Format("20060102")
	
	// Count returns created today for this organization
	err := r.db.Model(&domain.SalesReturn{}).
		Where("organization_id = ? AND return_number LIKE ?", orgID, "RMA-"+today+"%").
		Count(&count).Error
	if err != nil {
		return "", err
	}

	// Format: RMA-YYYYMMDD-XXXX
	return fmt.Sprintf("RMA-%s-%04d", today, count+1), nil
}
