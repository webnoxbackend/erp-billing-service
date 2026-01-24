package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/outbound"
	"erp-billing-service/internal/ports/repositories"
	shared_events "github.com/efs/shared-events"
)

// SalesOrderService handles sales order business logic
type SalesOrderService struct {
	salesOrderRepo repositories.SalesOrderRepository
	invoiceRepo    domain.InvoiceRepository
	eventPublisher domain.EventPublisher
	inventoryClient outbound.InventoryClient
}

// NewSalesOrderService creates a new sales order service
func NewSalesOrderService(
	salesOrderRepo repositories.SalesOrderRepository,
	invoiceRepo domain.InvoiceRepository,
	eventPublisher domain.EventPublisher,
	inventoryClient outbound.InventoryClient,
) *SalesOrderService {
	return &SalesOrderService{
		salesOrderRepo: salesOrderRepo,
		invoiceRepo:    invoiceRepo,
		eventPublisher: eventPublisher,
		inventoryClient: inventoryClient,
	}
}

// CreateSalesOrder creates a new sales order in draft status
func (s *SalesOrderService) CreateSalesOrder(req *dto.CreateSalesOrderRequest) (*dto.SalesOrderResponse, error) {
	// Check stock availability if inventory client is available
	if s.inventoryClient != nil {
		stockItems := make([]outbound.StockCheckItem, 0)
		for _, itemDTO := range req.Items {
			// Only check stock for GOODS items, assuming 'goods' type or if type is empty default to goods?
			// The ItemType isn't strictly defined here as enum, but let's assume we check everything for now or filter.
			// Ideally we check everything.
			stockItems = append(stockItems, outbound.StockCheckItem{
				ItemID:   itemDTO.ItemID.String(),
				Quantity: int32(itemDTO.Quantity),
			})
		}

		if len(stockItems) > 0 {
			unavailable, err := s.inventoryClient.CheckStockAvailability(context.Background(), stockItems)
			if err != nil {
				// Log error but maybe don't block if critical?
				// Plan says "Fail if unavailable".
				return nil, fmt.Errorf("failed to check stock availability: %w", err)
			}
			if len(unavailable) > 0 {
				// Construct error message with unavailable items
				errMsg := "stock unavailable for items: "
				for _, u := range unavailable {
					errMsg += fmt.Sprintf("%s (requested: %d, available: %d), ", u.ItemName, u.RequestedQuantity, u.AvailableQuantity)
				}
				return nil, fmt.Errorf(errMsg)
			}
		}
	}

	// Create sales order entity
	salesOrder := &domain.SalesOrder{
		ID:             uuid.New(),
		OrganizationID: req.OrganizationID,
		CustomerID:     req.CustomerID,
		ContactID:      req.ContactID,
		OrderDate:      req.OrderDate,
		Status:         domain.SalesOrderStatusDraft,
		TDSAmount:      req.TDSAmount,
		TCSAmount:      req.TCSAmount,
		Terms:          req.Terms,
		Notes:          req.Notes,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Add items
	for _, itemDTO := range req.Items {
		item := domain.SalesOrderItem{
			ID:           uuid.New(),
			SalesOrderID: salesOrder.ID,
			ItemID:       itemDTO.ItemID,
			ItemType:     itemDTO.ItemType,
			Name:         itemDTO.Name,
			Description:  itemDTO.Description,
			Quantity:     itemDTO.Quantity,
			UnitPrice:    itemDTO.UnitPrice,
			Discount:     itemDTO.Discount,
			Tax:          itemDTO.Tax,
			Total:        (itemDTO.Quantity * itemDTO.UnitPrice) - itemDTO.Discount + itemDTO.Tax,
			Metadata:     itemDTO.Metadata,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
		salesOrder.Items = append(salesOrder.Items, item)
	}

	// Calculate totals
	salesOrder.CalculateTotals()

	// Validate
	if err := salesOrder.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Save to database
	if err := s.salesOrderRepo.Create(salesOrder); err != nil {
		return nil, fmt.Errorf("failed to create sales order: %w", err)
	}

	// Update stock (Decrease)
	if s.inventoryClient != nil {
		stockItems := make([]outbound.StockCheckItem, 0)
		for _, item := range salesOrder.Items {
			stockItems = append(stockItems, outbound.StockCheckItem{
				ItemID:   item.ItemID.String(),
				Quantity: int32(item.Quantity),
			})
		}
		if len(stockItems) > 0 {
			// We use background context or pass context from request if available. 
			// CreateSalesOrder doesn't take context in current signature (bad practice but following existing code).
			// We'll use background.
			// TransactionType: 'sales', ReferenceType: 'sales_order'
			err := s.inventoryClient.UpdateStock(context.Background(), stockItems, "sales", "sales_order", salesOrder.ID.String(), "Sales Order Created")
			if err != nil {
				// If stock update fails, we might want to log it or alert.
				// Since we already saved the order, we shouldn't fail the request entirely, 
				// BUT consistency is broken.
				// For now, let's log and return error warning, or just return error.
				return nil, fmt.Errorf("sales order created but failed to update stock: %w", err)
			}
		}
	}

	// Publish event
	metadata := shared_events.NewEventMetadata(
		shared_events.EventType("sales_order.created"),
		shared_events.AggregateType("sales_order"),
		salesOrder.ID.String(),
	)
	event := domain.SalesOrderCreatedEvent{
		SalesOrderID: salesOrder.ID.String(),
		OrderNumber:  salesOrder.OrderNumber,
		CustomerID:   salesOrder.CustomerID.String(),
		TotalAmount:  salesOrder.TotalAmount,
		Status:       string(salesOrder.Status),
	}
	s.eventPublisher.Publish(context.Background(), metadata, event)

	return s.toSalesOrderResponse(salesOrder), nil
}

// UpdateSalesOrder updates a sales order (only in draft status)
func (s *SalesOrderService) UpdateSalesOrder(id uuid.UUID, req *dto.UpdateSalesOrderRequest) (*dto.SalesOrderResponse, error) {
	// Retrieve existing order
	salesOrder, err := s.salesOrderRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales order: %w", err)
	}

	// Check if can edit
	if !salesOrder.CanEdit() {
		return nil, fmt.Errorf("cannot edit sales order in %s status", salesOrder.Status)
	}

	// Update fields
	if req.CustomerID != nil {
		salesOrder.CustomerID = *req.CustomerID
	}
	if req.ContactID != nil {
		salesOrder.ContactID = req.ContactID
	}
	if req.OrderDate != nil {
		salesOrder.OrderDate = *req.OrderDate
	}
	if req.TDSAmount != nil {
		salesOrder.TDSAmount = *req.TDSAmount
	}
	if req.TCSAmount != nil {
		salesOrder.TCSAmount = *req.TCSAmount
	}
	if req.Terms != nil {
		salesOrder.Terms = *req.Terms
	}
	if req.Notes != nil {
		salesOrder.Notes = *req.Notes
	}

	// Update items if provided
	if req.Items != nil {
		salesOrder.Items = []domain.SalesOrderItem{}
		for _, itemDTO := range req.Items {
			item := domain.SalesOrderItem{
				ID:           uuid.New(),
				SalesOrderID: salesOrder.ID,
				ItemID:       itemDTO.ItemID,
				ItemType:     itemDTO.ItemType,
				Name:         itemDTO.Name,
				Description:  itemDTO.Description,
				Quantity:     itemDTO.Quantity,
				UnitPrice:    itemDTO.UnitPrice,
				Discount:     itemDTO.Discount,
				Tax:          itemDTO.Tax,
				Total:        (itemDTO.Quantity * itemDTO.UnitPrice) - itemDTO.Discount + itemDTO.Tax,
				Metadata:     itemDTO.Metadata,
				CreatedAt:    time.Now(),
				UpdatedAt:    time.Now(),
			}
			salesOrder.Items = append(salesOrder.Items, item)
		}
	}

	// Recalculate totals
	salesOrder.CalculateTotals()
	salesOrder.UpdatedAt = time.Now()

	// Validate
	if err := salesOrder.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Save
	if err := s.salesOrderRepo.Update(salesOrder); err != nil {
		return nil, fmt.Errorf("failed to update sales order: %w", err)
	}

	// Publish event
	metadata := shared_events.NewEventMetadata(
		shared_events.EventType("sales_order.updated"),
		shared_events.AggregateType("sales_order"),
		salesOrder.ID.String(),
	)
	event := domain.SalesOrderUpdatedEvent{
		SalesOrderID: salesOrder.ID.String(),
	}
	s.eventPublisher.Publish(context.Background(), metadata, event)

	return s.toSalesOrderResponse(salesOrder), nil
}

// ConfirmSalesOrder confirms a sales order and generates order number
func (s *SalesOrderService) ConfirmSalesOrder(id uuid.UUID) (*dto.SalesOrderResponse, error) {
	// Retrieve order
	salesOrder, err := s.salesOrderRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales order: %w", err)
	}

	// Check if can confirm
	if !salesOrder.CanConfirm() {
		return nil, fmt.Errorf("cannot confirm sales order in %s status", salesOrder.Status)
	}

	// Generate order number
	orderNumber, err := s.salesOrderRepo.GenerateOrderNumber(salesOrder.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate order number: %w", err)
	}
	salesOrder.OrderNumber = &orderNumber

	// Update status
	if err := salesOrder.CanTransitionTo(domain.SalesOrderStatusConfirmed); err != nil {
		return nil, err
	}
	salesOrder.Status = domain.SalesOrderStatusConfirmed
	salesOrder.UpdatedAt = time.Now()

	// Save
	if err := s.salesOrderRepo.Update(salesOrder); err != nil {
		return nil, fmt.Errorf("failed to confirm sales order: %w", err)
	}

	// Publish event
	metadata := shared_events.NewEventMetadata(
		shared_events.EventType("sales_order.confirmed"),
		shared_events.AggregateType("sales_order"),
		salesOrder.ID.String(),
	)
	event := domain.SalesOrderConfirmedEvent{
		SalesOrderID: salesOrder.ID.String(),
		OrderNumber:  *salesOrder.OrderNumber,
	}
	s.eventPublisher.Publish(context.Background(), metadata, event)

	return s.toSalesOrderResponse(salesOrder), nil
}

// CreateInvoiceFromOrder creates an invoice from a confirmed sales order
func (s *SalesOrderService) CreateInvoiceFromOrder(orderID uuid.UUID) (*dto.InvoiceResponse, error) {
	// Retrieve order
	salesOrder, err := s.salesOrderRepo.FindByID(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales order: %w", err)
	}

	// Check if can create invoice
	if !salesOrder.CanCreateInvoice() {
		return nil, fmt.Errorf("cannot create invoice for sales order in %s status or invoice already exists", salesOrder.Status)
	}

	// Create invoice entity
	invoice := &domain.Invoice{
		ID:             uuid.New(),
		OrganizationID: salesOrder.OrganizationID,
		CustomerID:     salesOrder.CustomerID,
		ContactID:      salesOrder.ContactID,
		Subject:        fmt.Sprintf("Invoice for Sales Order %s", *salesOrder.OrderNumber),
		SourceSystem:   domain.SourceSystemInventory,
		SalesOrderID:   &salesOrder.ID,
		InvoiceDate:    time.Now(),
		DueDate:        time.Now().AddDate(0, 0, 30), // 30 days default
		Status:         domain.InvoiceStatusDraft,
		SubTotal:       salesOrder.SubTotal,
		DiscountTotal:  salesOrder.DiscountTotal,
		TaxTotal:       salesOrder.TaxTotal,
		TDSAmount:      salesOrder.TDSAmount,
		TCSAmount:      salesOrder.TCSAmount,
		TotalAmount:    salesOrder.TotalAmount,
		BalanceAmount:  salesOrder.TotalAmount,
		Terms:          salesOrder.Terms,
		Notes:          salesOrder.Notes,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Copy items
	for _, orderItem := range salesOrder.Items {
		invoiceItem := domain.InvoiceItem{
			ID:          uuid.New(),
			InvoiceID:   invoice.ID,
			ItemID:      orderItem.ItemID,
			ItemType:    orderItem.ItemType,
			Name:        orderItem.Name,
			Description: orderItem.Description,
			Quantity:    orderItem.Quantity,
			UnitPrice:   orderItem.UnitPrice,
			Discount:    orderItem.Discount,
			Tax:         orderItem.Tax,
			Total:       orderItem.Total,
			Metadata:    orderItem.Metadata,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		invoice.Items = append(invoice.Items, invoiceItem)
	}

	// Save invoice
	if err := s.invoiceRepo.Create(context.Background(), invoice); err != nil {
		return nil, fmt.Errorf("failed to create invoice: %w", err)
	}

	// Update sales order with invoice reference
	salesOrder.InvoiceID = &invoice.ID
	salesOrder.Status = domain.SalesOrderStatusInvoiced
	salesOrder.UpdatedAt = time.Now()
	if err := s.salesOrderRepo.Update(salesOrder); err != nil {
		return nil, fmt.Errorf("failed to update sales order: %w", err)
	}

	// Publish events
	invoiceMeta := shared_events.NewEventMetadata(
		shared_events.EventType("billing.invoice.created"),
		shared_events.AggregateType("invoice"),
		invoice.ID.String(),
	)
	invoiceEvent := domain.InvoiceCreatedEvent{
		InvoiceID:  invoice.ID.String(),
		OrganizationID: invoice.OrganizationID.String(),
		CustomerID: invoice.CustomerID.String(),
		SourceSystem: string(invoice.SourceSystem),
		Subject: invoice.Subject,
		Status: string(invoice.Status),
		TotalAmount: invoice.TotalAmount,
		Currency: "USD",
		Timestamp: time.Now(),
	}
	s.eventPublisher.Publish(context.Background(), invoiceMeta, invoiceEvent)

	orderMeta := shared_events.NewEventMetadata(
		shared_events.EventType("sales_order.invoiced"),
		shared_events.AggregateType("sales_order"),
		salesOrder.ID.String(),
	)
	orderEvent := domain.SalesOrderInvoicedEvent{
		SalesOrderID: salesOrder.ID.String(),
		InvoiceID:    invoice.ID.String(),
	}
	s.eventPublisher.Publish(context.Background(), orderMeta, orderEvent)

	// Convert to response (simplified - you may need to fetch customer details)
	return &dto.InvoiceResponse{
		ID:            invoice.ID,
		InvoiceNumber: invoice.InvoiceNumber,
		SourceSystem:  string(invoice.SourceSystem),
		Subject:       invoice.Subject,
		Status:        string(invoice.Status),
		SubTotal:      invoice.SubTotal,
		DiscountTotal: invoice.DiscountTotal,
		TaxTotal:      invoice.TaxTotal,
		TotalAmount:   invoice.TotalAmount,
		PaidAmount:    invoice.PaidAmount,
		BalanceAmount: invoice.BalanceAmount,
		CustomerID:    invoice.CustomerID,
		ContactID:     invoice.ContactID,
		InvoiceDate:   invoice.InvoiceDate,
		DueDate:       invoice.DueDate,
	}, nil
}

// MarkAsShipped marks a sales order as shipped
func (s *SalesOrderService) MarkAsShipped(id uuid.UUID, req *dto.MarkAsShippedRequest) (*dto.SalesOrderResponse, error) {
	// Retrieve order
	salesOrder, err := s.salesOrderRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales order: %w", err)
	}

	// Check if can ship
	if !salesOrder.CanShip() {
		return nil, fmt.Errorf("cannot mark sales order as shipped in %s status or already shipped", salesOrder.Status)
	}

	// Update status
	if err := salesOrder.CanTransitionTo(domain.SalesOrderStatusShipped); err != nil {
		return nil, err
	}
	salesOrder.Status = domain.SalesOrderStatusShipped
	salesOrder.ShippedDate = &req.ShippedDate
	salesOrder.UpdatedAt = time.Now()

	// Save
	if err := s.salesOrderRepo.Update(salesOrder); err != nil {
		return nil, fmt.Errorf("failed to mark as shipped: %w", err)
	}

	// Publish event
	metadata := shared_events.NewEventMetadata(
		shared_events.EventType("sales_order.shipped"),
		shared_events.AggregateType("sales_order"),
		salesOrder.ID.String(),
	)
	event := domain.SalesOrderShippedEvent{
		SalesOrderID: salesOrder.ID.String(),
		ShippedDate:  req.ShippedDate,
	}
	s.eventPublisher.Publish(context.Background(), metadata, event)

	return s.toSalesOrderResponse(salesOrder), nil
}

// CancelSalesOrder cancels a sales order
func (s *SalesOrderService) CancelSalesOrder(id uuid.UUID, reason string) error {
	// Retrieve order
	salesOrder, err := s.salesOrderRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("failed to find sales order: %w", err)
	}

	// Check if can cancel
	if !salesOrder.CanCancel() {
		return fmt.Errorf("cannot cancel sales order in %s status", salesOrder.Status)
	}

	// Update status
	if err := salesOrder.CanTransitionTo(domain.SalesOrderStatusCancelled); err != nil {
		return err
	}
	salesOrder.Status = domain.SalesOrderStatusCancelled
	salesOrder.Notes = salesOrder.Notes + "\n\nCancellation Reason: " + reason
	salesOrder.UpdatedAt = time.Now()

	// Save
	if err := s.salesOrderRepo.Update(salesOrder); err != nil {
		return fmt.Errorf("failed to cancel sales order: %w", err)
	}

	// Publish event
	metadata := shared_events.NewEventMetadata(
		shared_events.EventType("sales_order.cancelled"),
		shared_events.AggregateType("sales_order"),
		salesOrder.ID.String(),
	)
	event := domain.SalesOrderCancelledEvent{
		SalesOrderID: salesOrder.ID.String(),
		Reason:       reason,
	}
	s.eventPublisher.Publish(context.Background(), metadata, event)

	return nil
}

// GetSalesOrder retrieves a sales order by ID
func (s *SalesOrderService) GetSalesOrder(id uuid.UUID) (*dto.SalesOrderResponse, error) {
	salesOrder, err := s.salesOrderRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales order: %w", err)
	}
	return s.toSalesOrderResponse(salesOrder), nil
}

// ListSalesOrders retrieves sales orders with filters
func (s *SalesOrderService) ListSalesOrders(orgID uuid.UUID, filters *dto.SalesOrderFilters) ([]*dto.SalesOrderResponse, int64, error) {
	salesOrders, total, err := s.salesOrderRepo.List(orgID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list sales orders: %w", err)
	}

	responses := make([]*dto.SalesOrderResponse, len(salesOrders))
	for i, order := range salesOrders {
		responses[i] = s.toSalesOrderResponse(order)
	}

	return responses, total, nil
}

// toSalesOrderResponse converts domain entity to DTO
func (s *SalesOrderService) toSalesOrderResponse(salesOrder *domain.SalesOrder) *dto.SalesOrderResponse {
	items := make([]dto.SalesOrderItemDTO, len(salesOrder.Items))
	for i, item := range salesOrder.Items {
		items[i] = dto.SalesOrderItemDTO{
			ID:          &item.ID,
			ItemID:      item.ItemID,
			ItemType:    item.ItemType,
			Name:        item.Name,
			Description: item.Description,
			Quantity:    item.Quantity,
			UnitPrice:   item.UnitPrice,
			Discount:    item.Discount,
			Tax:         item.Tax,
			Total:       item.Total,
			Metadata:    item.Metadata,
		}
	}

	return &dto.SalesOrderResponse{
		ID:             salesOrder.ID,
		OrganizationID: salesOrder.OrganizationID,
		CustomerID:     salesOrder.CustomerID,
		ContactID:      salesOrder.ContactID,
		OrderNumber:    salesOrder.OrderNumber,
		OrderDate:      salesOrder.OrderDate,
		Status:         string(salesOrder.Status),
		SubTotal:       salesOrder.SubTotal,
		DiscountTotal:  salesOrder.DiscountTotal,
		TaxTotal:       salesOrder.TaxTotal,
		TDSAmount:      salesOrder.TDSAmount,
		TCSAmount:      salesOrder.TCSAmount,
		TotalAmount:    salesOrder.TotalAmount,
		InvoiceID:      salesOrder.InvoiceID,
		ShippedDate:    salesOrder.ShippedDate,
		Terms:          salesOrder.Terms,
		Notes:          salesOrder.Notes,
		Items:          items,
		CreatedAt:      salesOrder.CreatedAt,
		UpdatedAt:      salesOrder.UpdatedAt,
	}
}
