package repositories

import (
	"github.com/google/uuid"
	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
)

// SalesReturnRepository defines the interface for sales return data access
type SalesReturnRepository interface {
	// Create creates a new sales return
	Create(salesReturn *domain.SalesReturn) error
	
	// Update updates an existing sales return
	Update(salesReturn *domain.SalesReturn) error
	
	// FindByID retrieves a sales return by ID with all relations
	FindByID(id uuid.UUID) (*domain.SalesReturn, error)
	
	// FindByReturnNumber retrieves a sales return by return number
	FindByReturnNumber(returnNumber string) (*domain.SalesReturn, error)
	
	// FindBySalesOrderID retrieves all sales returns for a sales order
	FindBySalesOrderID(orderID uuid.UUID) ([]*domain.SalesReturn, error)
	
	// List retrieves sales returns with filters and pagination
	List(orgID uuid.UUID, filters *dto.SalesReturnFilters) ([]*domain.SalesReturn, int64, error)
	
	// GenerateReturnNumber generates a unique return number for the organization
	GenerateReturnNumber(orgID uuid.UUID) (string, error)
}
