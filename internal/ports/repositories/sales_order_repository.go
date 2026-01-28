package repositories

import (
	"context"

	"github.com/google/uuid"
	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
)

// SalesOrderRepository defines the interface for sales order data access
type SalesOrderRepository interface {
	// Create creates a new sales order
	Create(order *domain.SalesOrder) error
	
	// Update updates an existing sales order
	Update(order *domain.SalesOrder) error
	
	// FindByID retrieves a sales order by ID with all relations
	FindByID(id uuid.UUID) (*domain.SalesOrder, error)
	
	// FindByOrderNumber retrieves a sales order by order number
	FindByOrderNumber(orderNumber string) (*domain.SalesOrder, error)
	
	// List retrieves sales orders with filters and pagination
	List(orgID uuid.UUID, filters *dto.SalesOrderFilters) ([]*domain.SalesOrder, int64, error)
	
	// Delete soft deletes a sales order
	Delete(id uuid.UUID) error
	
	// GenerateOrderNumber generates a unique order number for the organization
	GenerateOrderNumber(orgID uuid.UUID) (string, error)
	
	// FindByInvoiceID retrieves a sales order by invoice ID
	FindByInvoiceID(invoiceID uuid.UUID) (*domain.SalesOrder, error)

	// UpdateStatus updates the status of a sales order
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.SalesOrderStatus) error
}
