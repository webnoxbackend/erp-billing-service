package application

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/outbound"
	"erp-billing-service/internal/ports/repositories"
	"erp-billing-service/internal/validation"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
)

// SalesOrderService handles sales order business logic
type SalesOrderService struct {
	salesOrderRepo  repositories.SalesOrderRepository
	invoiceRepo     domain.InvoiceRepository
	rmRepo          domain.ReadModelRepository
	eventPublisher  domain.EventPublisher
	inventoryClient outbound.InventoryClient
	customerClient  outbound.CustomerClient
}

// NewSalesOrderService creates a new sales order service
func NewSalesOrderService(
	salesOrderRepo repositories.SalesOrderRepository,
	invoiceRepo domain.InvoiceRepository,
	rmRepo domain.ReadModelRepository,
	eventPublisher domain.EventPublisher,
	inventoryClient outbound.InventoryClient,
	customerClient outbound.CustomerClient,
) *SalesOrderService {
	return &SalesOrderService{
		salesOrderRepo:  salesOrderRepo,
		invoiceRepo:     invoiceRepo,
		rmRepo:          rmRepo,
		eventPublisher:  eventPublisher,
		inventoryClient: inventoryClient,
		customerClient:  customerClient,
	}
}

// CreateSalesOrder creates a new sales order in draft status
func (s *SalesOrderService) CreateSalesOrder(req *dto.CreateSalesOrderRequest) (*dto.SalesOrderResponse, error) {
	// Subscription limit validation
	subClient := validation.NewSubscriptionClient()
	if subClient != nil && req.OrganizationID != uuid.Nil {
		orgIDStr := req.OrganizationID.String()
		allowed, msg, err := subClient.ValidateRestriction(orgIDStr, validation.RestrictionKeyMaxSalesOrders)
		if err != nil {
			log.Printf("[Subscription] Validation error for org %s: %v — allowing (fail-open)", orgIDStr, err)
		} else if !allowed {
			return nil, fmt.Errorf("%s", msg)
		}
	}

	// Check stock availability if inventory client is available
	if s.inventoryClient != nil {
		stockItems := make([]outbound.StockCheckItem, 0)
		for _, itemDTO := range req.Items {
			// Only check stock for goods, product, or part items (case-insensitive)
			itemTypeLower := strings.ToLower(itemDTO.ItemType)
			if itemTypeLower == "goods" || itemTypeLower == "product" || itemTypeLower == "part" || itemTypeLower == "" {
				stockItems = append(stockItems, outbound.StockCheckItem{
					ItemID:   itemDTO.ItemID.String(),
					Quantity: int32(itemDTO.Quantity),
				})
			}
		}

		if len(stockItems) > 0 {
			unavailable, err := s.inventoryClient.CheckStockAvailability(context.Background(), stockItems)
			if err != nil {
				// If stock check fails (e.g. service down), log warning and proceed optimisticly
				fmt.Printf("Warning: failed to check stock availability: %v\n", err)
			} else if len(unavailable) > 0 {
				// Only fail if we explicitly know stock is unavailable
				errMsg := "stock unavailable for items: "
				for _, u := range unavailable {
					errMsg += fmt.Sprintf("%s (requested: %d, available: %d), ", u.ItemName, u.RequestedQuantity, u.AvailableQuantity)
				}
				return nil, fmt.Errorf("%s", errMsg)
			}
		}
	}

	// Create sales order entity
	orderID := uuid.New()
	draftNum := fmt.Sprintf("DRAFT-%s", orderID.String()[:8])
	salesOrder := &domain.SalesOrder{
		ID:                orderID,
		OrganizationID:    req.OrganizationID,
		CustomerID:        req.CustomerID,
		ContactID:         req.ContactID,
		OrderNumber:       &draftNum,
		ServiceCategoryID: req.ServiceCategoryID,
		PartCategoryID:    req.PartCategoryID,
		Subject:           req.Subject,
		OrderDate:         req.OrderDate,
		DueDate:           &req.DueDate,
		BillingAddressID:  req.BillingAddressID,
		ShippingAddressID: req.ShippingAddressID,
		ServiceAddressID:  req.ServiceAddressID,
		Status:            domain.SalesOrderStatusDraft,
		TDSPercentage:     req.TDSPercentage,
		TDSAmount:         req.TDSAmount,
		TCSPercentage:     req.TCSPercentage,
		TCSAmount:         req.TCSAmount,
		Adjustment:        req.Adjustment,
		ExciseDuty:        req.ExciseDuty,
		SalesCommission:   req.SalesCommission,
		Terms:             req.Terms,
		Notes:             req.Notes,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
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

	// Note: Stock is NOT reduced at creation time (draft status)
	// Stock will be reduced when the order is confirmed

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

	return s.toSalesOrderResponse(context.Background(), salesOrder), nil
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
	if req.ServiceCategoryID != nil {
		salesOrder.ServiceCategoryID = req.ServiceCategoryID
	}
	if req.PartCategoryID != nil {
		salesOrder.PartCategoryID = req.PartCategoryID
	}
	if req.BillingAddressID != nil {
		salesOrder.BillingAddressID = req.BillingAddressID
	}
	if req.ShippingAddressID != nil {
		salesOrder.ShippingAddressID = req.ShippingAddressID
	}
	if req.ServiceAddressID != nil {
		salesOrder.ServiceAddressID = req.ServiceAddressID
	}
	if req.OrderDate != nil {
		salesOrder.OrderDate = *req.OrderDate
	}
	if req.DueDate != nil {
		salesOrder.DueDate = req.DueDate
	}
	if req.Subject != nil {
		salesOrder.Subject = *req.Subject
	}
	if req.TDSPercentage != nil {
		salesOrder.TDSPercentage = *req.TDSPercentage
	}
	if req.TDSAmount != nil {
		salesOrder.TDSAmount = *req.TDSAmount
	}
	if req.TCSPercentage != nil {
		salesOrder.TCSPercentage = *req.TCSPercentage
	}
	if req.TCSAmount != nil {
		salesOrder.TCSAmount = *req.TCSAmount
	}
	if req.Adjustment != nil {
		salesOrder.Adjustment = *req.Adjustment
	}
	if req.ExciseDuty != nil {
		salesOrder.ExciseDuty = *req.ExciseDuty
	}
	if req.SalesCommission != nil {
		salesOrder.SalesCommission = *req.SalesCommission
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

	return s.toSalesOrderResponse(context.Background(), salesOrder), nil
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

	// Reduce stock when order is confirmed
	if s.inventoryClient != nil {
		stockItems := make([]outbound.StockCheckItem, 0)
		for _, item := range salesOrder.Items {
			// Only reduce stock for GOODS or PRODUCT items (case-insensitive)
			itemTypeLower := strings.ToLower(item.ItemType)
			if itemTypeLower == "goods" || itemTypeLower == "product" || itemTypeLower == "part" || itemTypeLower == "" {
				stockItems = append(stockItems, outbound.StockCheckItem{
					ItemID:   item.ItemID.String(),
					Quantity: int32(item.Quantity),
				})
			}
		}
		if len(stockItems) > 0 {
			err := s.inventoryClient.UpdateStock(context.Background(), stockItems, "sales", "sales_order", salesOrder.ID.String(), "Sales Order Confirmed")
			if err != nil {
				// If stock update fails, we should rollback the confirmation
				return nil, fmt.Errorf("failed to reduce stock for confirmed order: %w", err)
			}
		}
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

	return s.toSalesOrderResponse(context.Background(), salesOrder), nil
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

	var billingAddressID, shippingAddressID, serviceAddressID *uuid.UUID
	var billingStreet, billingCity, billingState, billingCode, billingCountry string
	var shippingStreet, shippingCity, shippingState, shippingCode, shippingCountry string

	billingAddressID = salesOrder.BillingAddressID
	shippingAddressID = salesOrder.ShippingAddressID
	serviceAddressID = salesOrder.ServiceAddressID

	if customer, err := s.rmRepo.GetCustomer(context.Background(), salesOrder.CustomerID); err == nil && customer != nil {
		if billingAddressID == nil {
			billingAddressID = customer.BillingAddressID
		}
		if shippingAddressID == nil {
			shippingAddressID = customer.ShippingAddressID
		}
		if serviceAddressID == nil {
			serviceAddressID = customer.ServiceAddressID
		}
		billingStreet = customer.BillingStreet
		billingCity = customer.BillingCity
		billingState = customer.BillingState
		billingCode = customer.BillingCode
		billingCountry = customer.BillingCountry
		shippingStreet = customer.ShippingStreet
		shippingCity = customer.ShippingCity
		shippingState = customer.ShippingState
		shippingCode = customer.ShippingCode
		shippingCountry = customer.ShippingCountry
	}

	if billingAddressID != nil {
		if addr, err := s.rmRepo.GetAddress(context.Background(), *billingAddressID); err == nil && addr != nil {
			billingStreet = addr.Street1
			if addr.Street2 != "" {
				billingStreet = addr.Street1 + ", " + addr.Street2
			}
			billingCity = addr.City
			billingState = addr.State
			billingCode = addr.PostalCode
			billingCountry = addr.Country
		}
	}
	if shippingAddressID != nil {
		if addr, err := s.rmRepo.GetAddress(context.Background(), *shippingAddressID); err == nil && addr != nil {
			shippingStreet = addr.Street1
			if addr.Street2 != "" {
				shippingStreet = addr.Street1 + ", " + addr.Street2
			}
			shippingCity = addr.City
			shippingState = addr.State
			shippingCode = addr.PostalCode
			shippingCountry = addr.Country
		}
	}

	// Create invoice entity
	invoiceSubject := salesOrder.Subject
	if invoiceSubject == "" {
		if salesOrder.OrderNumber != nil {
			invoiceSubject = fmt.Sprintf("Sales Order %s", *salesOrder.OrderNumber)
		} else {
			invoiceSubject = fmt.Sprintf("Sales Order %s", salesOrder.ID.String()[:8])
		}
	}

	invoice := &domain.Invoice{
		ID:             uuid.New(),
		OrganizationID: salesOrder.OrganizationID,
		CustomerID:     salesOrder.CustomerID,
		ContactID:      salesOrder.ContactID,
		Subject:        invoiceSubject,
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
		Adjustment:      salesOrder.Adjustment,
		ExciseDuty:      salesOrder.ExciseDuty,
		SalesCommission: salesOrder.SalesCommission,
		Terms:             salesOrder.Terms,
		Notes:             salesOrder.Notes,
		ServiceCategoryID: salesOrder.ServiceCategoryID,
		PartCategoryID:    salesOrder.PartCategoryID,
		BillingAddressID:  billingAddressID,
		ShippingAddressID: shippingAddressID,
		ServiceAddressID:  serviceAddressID,
		BillingStreet:     billingStreet,
		BillingCity:       billingCity,
		BillingState:      billingState,
		BillingCode:       billingCode,
		BillingCountry:    billingCountry,
		ShippingStreet:    shippingStreet,
		ShippingCity:      shippingCity,
		ShippingState:     shippingState,
		ShippingCode:      shippingCode,
		ShippingCountry:   shippingCountry,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Copy items
	for _, orderItem := range salesOrder.Items {
		itemType := "service"
		itemTypeLower := strings.ToLower(orderItem.ItemType)
		if itemTypeLower == "goods" || itemTypeLower == "product" || itemTypeLower == "part" {
			itemType = "part"
		}

		invoiceItem := domain.InvoiceItem{
			ID:          uuid.New(),
			InvoiceID:   invoice.ID,
			ItemID:      orderItem.ItemID,
			ItemType:    itemType,
			Name:        orderItem.Name,
			Description: orderItem.Description,
			Quantity:    orderItem.Quantity,
			UnitPrice:   orderItem.UnitPrice,
			Discount:    orderItem.Discount,
			Tax:         orderItem.Tax,
			Total:       orderItem.Total,
			Metadata:          orderItem.Metadata,
			ServiceCategoryID: salesOrder.ServiceCategoryID,
			PartCategoryID:    salesOrder.PartCategoryID,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
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
		InvoiceID:      invoice.ID.String(),
		OrganizationID: invoice.OrganizationID.String(),
		CustomerID:     invoice.CustomerID.String(),
		SourceSystem:   string(invoice.SourceSystem),
		Subject:        invoice.Subject,
		Status:         string(invoice.Status),
		TotalAmount:    invoice.TotalAmount,
		Currency:       "USD",
		Timestamp:      time.Now(),
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
		ContactID:         invoice.ContactID,
		ServiceCategoryID: invoice.ServiceCategoryID,
		PartCategoryID:    invoice.PartCategoryID,
		Adjustment:        invoice.Adjustment,
		ExciseDuty:        invoice.ExciseDuty,
		SalesCommission:   invoice.SalesCommission,
		InvoiceDate:       invoice.InvoiceDate,
		DueDate:           invoice.DueDate,
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

	return s.toSalesOrderResponse(context.Background(), salesOrder), nil
}

// MarkAsDelivered marks a sales order as delivered
func (s *SalesOrderService) MarkAsDelivered(id uuid.UUID, req *dto.MarkAsDeliveredRequest) (*dto.SalesOrderResponse, error) {
	// Retrieve order
	salesOrder, err := s.salesOrderRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales order: %w", err)
	}

	// Check if can deliver
	if !salesOrder.CanDeliver() {
		return nil, fmt.Errorf("cannot mark sales order as delivered in %s status or already delivered", salesOrder.Status)
	}

	// Update status
	if err := salesOrder.CanTransitionTo(domain.SalesOrderStatusDelivered); err != nil {
		return nil, err
	}
	salesOrder.Status = domain.SalesOrderStatusDelivered
	salesOrder.DeliveredDate = &req.DeliveredDate
	salesOrder.UpdatedAt = time.Now()

	// Save
	if err := s.salesOrderRepo.Update(salesOrder); err != nil {
		return nil, fmt.Errorf("failed to mark as delivered: %w", err)
	}

	// Publish event
	metadata := shared_events.NewEventMetadata(
		shared_events.EventType("sales_order.delivered"),
		shared_events.AggregateType("sales_order"),
		salesOrder.ID.String(),
	)
	event := domain.SalesOrderDeliveredEvent{
		SalesOrderID:  salesOrder.ID.String(),
		DeliveredDate: req.DeliveredDate,
	}
	s.eventPublisher.Publish(context.Background(), metadata, event)

	return s.toSalesOrderResponse(context.Background(), salesOrder), nil
}

// DeleteSalesOrder soft-deletes a sales order
func (s *SalesOrderService) DeleteSalesOrder(id uuid.UUID) error {
	salesOrder, err := s.salesOrderRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("failed to find sales order: %w", err)
	}

	// Restore stock if order was confirmed or shipped before deleting
	if salesOrder.Status == domain.SalesOrderStatusConfirmed || salesOrder.Status == domain.SalesOrderStatusShipped {
		if s.inventoryClient != nil {
			stockItems := make([]outbound.StockCheckItem, 0)
			for _, item := range salesOrder.Items {
				itemTypeLower := strings.ToLower(item.ItemType)
				if itemTypeLower == "goods" || itemTypeLower == "product" || itemTypeLower == "part" || itemTypeLower == "" {
					stockItems = append(stockItems, outbound.StockCheckItem{
						ItemID:   item.ItemID.String(),
						Quantity: int32(item.Quantity),
					})
				}
			}
			if len(stockItems) > 0 {
				_ = s.inventoryClient.UpdateStock(context.Background(), stockItems, "return", "sales_order_deletion", salesOrder.ID.String(), "Sales Order Deleted - Stock Restored")
			}
		}
	}

	return s.salesOrderRepo.Delete(id)
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

	// Restore stock if order was confirmed (stock was already reduced)
	if salesOrder.Status == domain.SalesOrderStatusConfirmed || salesOrder.Status == domain.SalesOrderStatusShipped {
		if s.inventoryClient != nil {
			stockItems := make([]outbound.StockCheckItem, 0)
			for _, item := range salesOrder.Items {
				// Only restore stock for GOODS or PRODUCT items (case-insensitive)
				itemTypeLower := strings.ToLower(item.ItemType)
				if itemTypeLower == "goods" || itemTypeLower == "product" || itemTypeLower == "part" || itemTypeLower == "" {
					stockItems = append(stockItems, outbound.StockCheckItem{
						ItemID:   item.ItemID.String(),
						Quantity: int32(item.Quantity),
					})
				}
			}
			if len(stockItems) > 0 {
				// Use "return" transaction type to add stock back
				err := s.inventoryClient.UpdateStock(context.Background(), stockItems, "return", "sales_order_cancellation", salesOrder.ID.String(), "Sales Order Cancelled - Stock Restored")
				if err != nil {
					// Log error but don't fail cancellation
					// This is a compensating transaction, manual intervention may be needed
					fmt.Printf("Warning: failed to restore stock for cancelled order %s: %v\n", salesOrder.ID.String(), err)
				}
			}
		}
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
	return s.toSalesOrderResponse(context.Background(), salesOrder), nil
}

// ListSalesOrders retrieves sales orders with filters
func (s *SalesOrderService) ListSalesOrders(orgID uuid.UUID, filters *dto.SalesOrderFilters) ([]*dto.SalesOrderResponse, int64, error) {
	salesOrders, total, err := s.salesOrderRepo.List(orgID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list sales orders: %w", err)
	}

	responses := make([]*dto.SalesOrderResponse, len(salesOrders))
	for i, order := range salesOrders {
		responses[i] = s.toSalesOrderResponse(context.Background(), order)
	}

	return responses, total, nil
}

// toSalesOrderResponse converts domain entity to DTO
func (s *SalesOrderService) toSalesOrderResponse(ctx context.Context, salesOrder *domain.SalesOrder) *dto.SalesOrderResponse {
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

	// Fetch customer name from read model
	var customerName string
	var companyName string
	var subject string
	var billingStreet, billingCity, billingState, billingCode, billingCountry string
	var shippingStreet, shippingCity, shippingState, shippingCode, shippingCountry string

	customer, err := s.rmRepo.GetCustomer(ctx, salesOrder.CustomerID)
	if err != nil || customer == nil || customer.DisplayName == "" {
		// Fallback to customerClient if read model fails or provides empty name
		if s.customerClient != nil {
			fmt.Printf("[INFO] SalesOrder customer %s missing or invalid in ReadModel, fetching from Customer Service\n", salesOrder.CustomerID)
			remoteCustomer, remoteErr := s.customerClient.GetCustomer(ctx, salesOrder.CustomerID)
			if remoteErr == nil && remoteCustomer != nil {
				customerName = remoteCustomer.DisplayName
				companyName = remoteCustomer.CompanyName
				billingStreet = remoteCustomer.BillingStreet
				billingCity = remoteCustomer.BillingCity
				billingState = remoteCustomer.BillingState
				billingCode = remoteCustomer.BillingCode
				billingCountry = remoteCustomer.BillingCountry
				shippingStreet = remoteCustomer.ShippingStreet
				shippingCity = remoteCustomer.ShippingCity
				shippingState = remoteCustomer.ShippingState
				shippingCode = remoteCustomer.ShippingCode
				shippingCountry = remoteCustomer.ShippingCountry
			}
		}

		// If still empty after fallback, use "Unknown Customer"
		if customerName == "" {
			customerName = "Unknown Customer"
		}
	} else {
		customerName = customer.DisplayName
		companyName = customer.CompanyName
		billingStreet = customer.BillingStreet
		billingCity = customer.BillingCity
		billingState = customer.BillingState
		billingCode = customer.BillingCode
		billingCountry = customer.BillingCountry
		shippingStreet = customer.ShippingStreet
		shippingCity = customer.ShippingCity
		shippingState = customer.ShippingState
		shippingCode = customer.ShippingCode
		shippingCountry = customer.ShippingCountry
	}

	// Override/Load addresses from individual IDs if set
	if salesOrder.BillingAddressID != nil {
		if addr, err := s.rmRepo.GetAddress(ctx, *salesOrder.BillingAddressID); err == nil && addr != nil {
			billingStreet = addr.Street1
			if addr.Street2 != "" {
				billingStreet = addr.Street1 + ", " + addr.Street2
			}
			billingCity = addr.City
			billingState = addr.State
			billingCode = addr.PostalCode
			billingCountry = addr.Country
		}
	}
	if salesOrder.ShippingAddressID != nil {
		if addr, err := s.rmRepo.GetAddress(ctx, *salesOrder.ShippingAddressID); err == nil && addr != nil {
			shippingStreet = addr.Street1
			if addr.Street2 != "" {
				shippingStreet = addr.Street1 + ", " + addr.Street2
			}
			shippingCity = addr.City
			shippingState = addr.State
			shippingCode = addr.PostalCode
			shippingCountry = addr.Country
		}
	}

	// Fetch contact info if available
	var contact *dto.ContactResponse
	if salesOrder.ContactID != nil {
		c, err := s.rmRepo.GetContact(ctx, *salesOrder.ContactID)
		if err == nil && c != nil && c.FirstName != "" {
			contact = &dto.ContactResponse{
				ID:        c.ID,
				FirstName: c.FirstName,
				LastName:  c.LastName,
				Email:     c.Email,
			}
		} else if s.customerClient != nil {
			// Fallback to customerClient
			remoteContact, remoteErr := s.customerClient.GetContact(ctx, *salesOrder.ContactID)
			if remoteErr == nil && remoteContact != nil {
				contact = &dto.ContactResponse{
					ID:        remoteContact.ID,
					FirstName: remoteContact.FirstName,
					LastName:  remoteContact.LastName,
					Email:     remoteContact.Email,
				}
			}
		}
	}

	// Generate subject from order number or use a default if not set
	if salesOrder.Subject != "" {
		subject = salesOrder.Subject
	} else if salesOrder.OrderNumber != nil {
		subject = fmt.Sprintf("Sales Order %s", *salesOrder.OrderNumber)
	} else {
		subject = fmt.Sprintf("Draft Sales Order for %s", customerName)
	}

	return &dto.SalesOrderResponse{
		ID:             salesOrder.ID,
		OrganizationID: salesOrder.OrganizationID,
		CustomerID:     salesOrder.CustomerID,
		CustomerName:   customerName,
		ContactID:      salesOrder.ContactID,
		OrderNumber:    salesOrder.OrderNumber,
		Subject:        subject,
		OrderDate:      ConvertToOrgTZValue(ctx, salesOrder.OrderDate, salesOrder.OrganizationID, s.rmRepo),
		Status:         string(salesOrder.Status),
		SubTotal:       salesOrder.SubTotal,
		DiscountTotal:  salesOrder.DiscountTotal,
		TaxTotal:       salesOrder.TaxTotal,
		TDSPercentage:   salesOrder.TDSPercentage,
		TDSAmount:      salesOrder.TDSAmount,
		TCSPercentage:   salesOrder.TCSPercentage,
		TCSAmount:      salesOrder.TCSAmount,
		Adjustment:      salesOrder.Adjustment,
		ExciseDuty:      salesOrder.ExciseDuty,
		SalesCommission: salesOrder.SalesCommission,
		TotalAmount:    salesOrder.TotalAmount,
		InvoiceID:         salesOrder.InvoiceID,
		ServiceCategoryID: salesOrder.ServiceCategoryID,
		PartCategoryID:    salesOrder.PartCategoryID,
		BillingAddressID:  salesOrder.BillingAddressID,
		ShippingAddressID: salesOrder.ShippingAddressID,
		ServiceAddressID:  salesOrder.ServiceAddressID,
		ShippedDate:       ConvertToOrgTZ(ctx, salesOrder.ShippedDate, salesOrder.OrganizationID, s.rmRepo),
		DeliveredDate:     ConvertToOrgTZ(ctx, salesOrder.DeliveredDate, salesOrder.OrganizationID, s.rmRepo),
		DueDate:           ConvertToOrgTZ(ctx, salesOrder.DueDate, salesOrder.OrganizationID, s.rmRepo),
		Terms:          salesOrder.Terms,
		Notes:          salesOrder.Notes,
		Items:          items,
		CreatedAt:      ConvertToOrgTZValue(ctx, salesOrder.CreatedAt, salesOrder.OrganizationID, s.rmRepo),
		UpdatedAt:      ConvertToOrgTZValue(ctx, salesOrder.UpdatedAt, salesOrder.OrganizationID, s.rmRepo),
		Customer: &dto.CustomerResponse{
			ID:          salesOrder.CustomerID,
			DisplayName: customerName,
			CompanyName: companyName,
		},
		Contact:         contact,
		BillingStreet:   billingStreet,
		BillingCity:     billingCity,
		BillingState:    billingState,
		BillingCode:     billingCode,
		BillingCountry:  billingCountry,
		ShippingStreet:  shippingStreet,
		ShippingCity:    shippingCity,
		ShippingState:   shippingState,
		ShippingCode:    shippingCode,
		ShippingCountry: shippingCountry,
	}
}
