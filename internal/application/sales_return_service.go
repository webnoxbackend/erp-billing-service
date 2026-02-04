package application

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/outbound"
	"erp-billing-service/internal/ports/repositories"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
)

// SalesReturnService handles sales return business logic
type SalesReturnService struct {
	salesReturnRepo repositories.SalesReturnRepository
	salesOrderRepo  repositories.SalesOrderRepository
	invoiceRepo     domain.InvoiceRepository
	paymentRepo     domain.PaymentRepository
	eventPublisher  domain.EventPublisher
	inventoryClient outbound.InventoryClient
}

// NewSalesReturnService creates a new sales return service
func NewSalesReturnService(
	salesReturnRepo repositories.SalesReturnRepository,
	salesOrderRepo repositories.SalesOrderRepository,
	invoiceRepo domain.InvoiceRepository,
	paymentRepo domain.PaymentRepository,
	eventPublisher domain.EventPublisher,
	inventoryClient outbound.InventoryClient,
) *SalesReturnService {
	return &SalesReturnService{
		salesReturnRepo: salesReturnRepo,
		salesOrderRepo:  salesOrderRepo,
		invoiceRepo:     invoiceRepo,
		paymentRepo:     paymentRepo,
		eventPublisher:  eventPublisher,
		inventoryClient: inventoryClient,
	}
}

// CreateSalesReturn creates and approves a new sales return
func (s *SalesReturnService) CreateSalesReturn(req *dto.CreateSalesReturnRequest) (*dto.SalesReturnResponse, error) {
	// Retrieve sales order
	salesOrder, err := s.salesOrderRepo.FindByID(req.SalesOrderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales order: %w", err)
	}

	// Validate order can be returned (must be paid and shipped)
	if !salesOrder.CanReturn() {
		return nil, fmt.Errorf("sales order must be paid and shipped to create a return (current status: %s, shipped: %v)",
			salesOrder.Status, salesOrder.ShippedDate != nil)
	}

	// Create sales return entity
	salesReturn := &domain.SalesReturn{
		ID:             uuid.New(),
		OrganizationID: req.OrganizationID,
		SalesOrderID:   req.SalesOrderID,
		ReturnDate:     req.ReturnDate,
		Status:         domain.SalesReturnStatusDraft,
		ReturnReason:   req.ReturnReason,
		Notes:          req.Notes,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Add return items
	for _, itemDTO := range req.Items {
		item := domain.SalesReturnItem{
			ID:               uuid.New(),
			SalesReturnID:    salesReturn.ID,
			SalesOrderItemID: itemDTO.SalesOrderItemID,
			ReturnedQuantity: itemDTO.ReturnedQuantity,
			UnitPrice:        itemDTO.UnitPrice,
			Tax:              itemDTO.Tax,
			Total:            (itemDTO.ReturnedQuantity * itemDTO.UnitPrice) + itemDTO.Tax,
			Reason:           itemDTO.Reason,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}
		salesReturn.Items = append(salesReturn.Items, item)
	}

	// Calculate totals
	salesReturn.CalculateTotals()

	// Validate return quantities
	if err := salesReturn.ValidateReturnQuantity(salesOrder.Items); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Validate
	if err := salesReturn.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Generate return number and approve immediately
	returnNumber, err := s.salesReturnRepo.GenerateReturnNumber(salesReturn.OrganizationID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate return number: %w", err)
	}
	salesReturn.ReturnNumber = &returnNumber
	salesReturn.Status = domain.SalesReturnStatusApproved
	approvedDate := time.Now()
	salesReturn.ApprovedDate = &approvedDate

	// Save to database
	if err := s.salesReturnRepo.Create(salesReturn); err != nil {
		return nil, fmt.Errorf("failed to create sales return: %w", err)
	}

	// Publish events
	event := domain.SalesReturnCreatedEvent{
		SalesReturnID: salesReturn.ID.String(),
		ReturnNumber:  *salesReturn.ReturnNumber,
		SalesOrderID:  salesReturn.SalesOrderID.String(),
		ReturnAmount:  salesReturn.ReturnAmount,
	}
	s.eventPublisher.Publish(context.Background(), shared_events.NewEventMetadata(shared_events.EventType("sales_return.created"), shared_events.AggregateType("sales_return"), salesReturn.ID.String()), event)

	return s.toSalesReturnResponse(salesReturn), nil
}

// ReceiveReturn marks a sales return as received
func (s *SalesReturnService) ReceiveReturn(id uuid.UUID, req *dto.ReceiveReturnRequest) (*dto.SalesReturnResponse, error) {
	// Retrieve return
	salesReturn, err := s.salesReturnRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales return: %w", err)
	}

	// Check if can receive
	if !salesReturn.CanReceive() {
		return nil, fmt.Errorf("cannot receive sales return in %s status", salesReturn.Status)
	}

	// Update status
	if err := salesReturn.CanTransitionTo(domain.SalesReturnStatusReceived); err != nil {
		return nil, err
	}
	salesReturn.Status = domain.SalesReturnStatusReceived
	salesReturn.ReceivedDate = &req.ReceivedDate
	salesReturn.ReceivingNotes = req.ReceivingNotes
	salesReturn.UpdatedAt = time.Now()

	// Save
	if err := s.salesReturnRepo.Update(salesReturn); err != nil {
		return nil, fmt.Errorf("failed to receive sales return: %w", err)
	}

	// Publish event
	event := domain.SalesReturnReceivedEvent{
		SalesReturnID: salesReturn.ID.String(),
		ReceivedDate:  req.ReceivedDate,
	}
	s.eventPublisher.Publish(context.Background(), shared_events.NewEventMetadata(shared_events.EventType("sales_return.received"), shared_events.AggregateType("sales_return"), salesReturn.ID.String()), event)

	return s.toSalesReturnResponse(salesReturn), nil
}

// ProcessRefund processes a refund payment for a sales return
func (s *SalesReturnService) ProcessRefund(id uuid.UUID, req *dto.ProcessRefundRequest) (*dto.SalesReturnResponse, error) {
	fmt.Printf("[DEBUG] ProcessRefund called for sales return ID: %s\n", id)

	// Retrieve return
	salesReturn, err := s.salesReturnRepo.FindByID(id)
	if err != nil {
		fmt.Printf("[ERROR] Failed to find sales return: %v\n", err)
		return nil, fmt.Errorf("failed to find sales return: %w", err)
	}
	fmt.Printf("[DEBUG] Sales return found: status=%s, refund_payment_id=%v\n", salesReturn.Status, salesReturn.RefundPaymentID)

	// Check if can refund
	if !salesReturn.CanRefund() {
		fmt.Printf("[ERROR] Cannot refund: status=%s, refund_payment_id=%v\n", salesReturn.Status, salesReturn.RefundPaymentID)
		return nil, fmt.Errorf("cannot process refund for sales return in %s status or refund already processed", salesReturn.Status)
	}

	// Retrieve sales order to get invoice
	salesOrder, err := s.salesOrderRepo.FindByID(salesReturn.SalesOrderID)
	if err != nil {
		fmt.Printf("[ERROR] Failed to find sales order: %v\n", err)
		return nil, fmt.Errorf("failed to find sales order: %w", err)
	}
	fmt.Printf("[DEBUG] Sales order found: invoice_id=%v\n", salesOrder.InvoiceID)

	if salesOrder.InvoiceID == nil {
		fmt.Printf("[ERROR] Sales order has no associated invoice\n")
		return nil, fmt.Errorf("sales order has no associated invoice")
	}

	// Retrieve invoice
	invoice, err := s.invoiceRepo.GetByID(context.Background(), *salesOrder.InvoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to find invoice: %w", err)
	}

	// Check if invoice can be refunded
	if !invoice.CanRefund() {
		return nil, fmt.Errorf("invoice must be paid to process refund")
	}

	// Create refund payment (negative amount)
	refundPayment := &domain.Payment{
		ID:             uuid.New(),
		OrganizationID: salesReturn.OrganizationID,
		InvoiceID:      invoice.ID,
		Amount:         -salesReturn.ReturnAmount, // Negative for refund
		PaymentDate:    req.PaymentDate,
		Method:         domain.PaymentMethod(req.Method),
		Reference:      req.Reference,
		Status:         domain.PaymentStatusCompleted,
		PaymentType:    domain.PaymentTypeRefund,
		SalesReturnID:  &salesReturn.ID,
		Notes:          req.Notes,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Save payment
	if err := s.paymentRepo.Create(context.Background(), refundPayment); err != nil {
		return nil, fmt.Errorf("failed to create refund payment: %w", err)
	}

	// Update invoice balance
	invoice.PaidAmount -= salesReturn.ReturnAmount
	invoice.BalanceAmount += salesReturn.ReturnAmount
	invoice.UpdatedAt = time.Now()
	if err := s.invoiceRepo.Update(context.Background(), invoice); err != nil {
		return nil, fmt.Errorf("failed to update invoice: %w", err)
	}

	// Update sales return
	if err := salesReturn.CanTransitionTo(domain.SalesReturnStatusRefunded); err != nil {
		return nil, err
	}
	salesReturn.Status = domain.SalesReturnStatusRefunded
	salesReturn.RefundPaymentID = &refundPayment.ID
	refundedDate := time.Now()
	salesReturn.RefundedDate = &refundedDate
	salesReturn.UpdatedAt = time.Now()

	// Save
	if err := s.salesReturnRepo.Update(salesReturn); err != nil {
		return nil, fmt.Errorf("failed to update sales return: %w", err)
	}

	// Update stock (Increase)
	if s.inventoryClient != nil {
		stockItems := make([]outbound.StockCheckItem, 0)
		// Based on `CreateSalesReturn` (lines 69-83), it only maps `SalesOrderItemID`.
		// It assumes `SalesReturnItem` struct has `SalesOrderItemID` but maybe not `ItemID`.
		// If I cannot get ItemID easily, I have to fetch SalesOrder items again.
		// I have `salesOrder` in `ProcessRefund`.
		// I can map SalesOrderItemID -> ItemID using `salesOrder.Items`.
		// Let's do that.

		for _, returnItem := range salesReturn.Items {
			var itemID string
			// Find corresponding item in salesOrder
			for _, soItem := range salesOrder.Items {
				if soItem.ID == returnItem.SalesOrderItemID {
					itemID = soItem.ItemID.String()
					break
				}
			}
			if itemID != "" {
				stockItems = append(stockItems, outbound.StockCheckItem{
					ItemID:   itemID,
					Quantity: int32(returnItem.ReturnedQuantity),
				})
			}
		}

		if len(stockItems) > 0 {
			// TransactionType: 'return', ReferenceType: 'sales_return'
			err := s.inventoryClient.UpdateStock(context.Background(), stockItems, "return", "sales_return", salesReturn.ID.String(), "Sales Return Refunded")
			if err != nil {
				// Log warning
				fmt.Printf("sales return refunded but failed to update stock: %v\n", err)
			}
		}
	}

	// Publish events
	paymentEvent := domain.PaymentRecordedEvent{
		PaymentID:   refundPayment.ID.String(),
		InvoiceID:   invoice.ID.String(),
		Amount:      refundPayment.Amount,
		PaymentType: string(refundPayment.PaymentType),
	}
	s.eventPublisher.Publish(context.Background(), shared_events.NewEventMetadata(shared_events.EventType("billing.payment.refund.recorded"), shared_events.AggregateType("payment"), refundPayment.ID.String()), paymentEvent)

	returnEvent := domain.SalesReturnRefundedEvent{
		SalesReturnID: salesReturn.ID.String(),
		RefundAmount:  salesReturn.ReturnAmount,
		PaymentID:     refundPayment.ID.String(),
	}
	s.eventPublisher.Publish(context.Background(), shared_events.NewEventMetadata(shared_events.EventType("sales_return.refunded"), shared_events.AggregateType("sales_return"), salesReturn.ID.String()), returnEvent)

	return s.toSalesReturnResponse(salesReturn), nil
}

// GetSalesReturn retrieves a sales return by ID
func (s *SalesReturnService) GetSalesReturn(id uuid.UUID) (*dto.SalesReturnResponse, error) {
	salesReturn, err := s.salesReturnRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales return: %w", err)
	}
	return s.toSalesReturnResponse(salesReturn), nil
}

// ListSalesReturns retrieves sales returns with filters
func (s *SalesReturnService) ListSalesReturns(orgID uuid.UUID, filters *dto.SalesReturnFilters) ([]*dto.SalesReturnResponse, int64, error) {
	salesReturns, total, err := s.salesReturnRepo.List(orgID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list sales returns: %w", err)
	}

	responses := make([]*dto.SalesReturnResponse, len(salesReturns))
	for i, salesReturn := range salesReturns {
		responses[i] = s.toSalesReturnResponse(salesReturn)
	}

	return responses, total, nil
}

// GetReturnsBySalesOrder retrieves all returns for a sales order
func (s *SalesReturnService) GetReturnsBySalesOrder(orderID uuid.UUID) ([]*dto.SalesReturnResponse, error) {
	salesReturns, err := s.salesReturnRepo.FindBySalesOrderID(orderID)
	if err != nil {
		return nil, fmt.Errorf("failed to find sales returns: %w", err)
	}

	responses := make([]*dto.SalesReturnResponse, len(salesReturns))
	for i, salesReturn := range salesReturns {
		responses[i] = s.toSalesReturnResponse(salesReturn)
	}

	return responses, nil
}

// toSalesReturnResponse converts domain entity to DTO
func (s *SalesReturnService) toSalesReturnResponse(salesReturn *domain.SalesReturn) *dto.SalesReturnResponse {
	items := make([]dto.SalesReturnItemDTO, len(salesReturn.Items))
	for i, item := range salesReturn.Items {
		items[i] = dto.SalesReturnItemDTO{
			ID:               &item.ID,
			SalesOrderItemID: item.SalesOrderItemID,
			ReturnedQuantity: item.ReturnedQuantity,
			UnitPrice:        item.UnitPrice,
			Tax:              item.Tax,
			Total:            item.Total,
			Reason:           item.Reason,
		}
	}

	return &dto.SalesReturnResponse{
		ID:              salesReturn.ID,
		OrganizationID:  salesReturn.OrganizationID,
		SalesOrderID:    salesReturn.SalesOrderID,
		ReturnNumber:    salesReturn.ReturnNumber,
		ReturnDate:      salesReturn.ReturnDate,
		Status:          string(salesReturn.Status),
		ReturnAmount:    salesReturn.ReturnAmount,
		ReturnReason:    salesReturn.ReturnReason,
		Notes:           salesReturn.Notes,
		ApprovedDate:    salesReturn.ApprovedDate,
		ReceivedDate:    salesReturn.ReceivedDate,
		ReceivingNotes:  salesReturn.ReceivingNotes,
		RefundedDate:    salesReturn.RefundedDate,
		RefundPaymentID: salesReturn.RefundPaymentID,
		Items:           items,
		CreatedAt:       salesReturn.CreatedAt,
		UpdatedAt:       salesReturn.UpdatedAt,
	}
}
