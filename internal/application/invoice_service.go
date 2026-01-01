package application

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
)

type InvoiceService struct {
	invoiceRepo    domain.InvoiceRepository
	rmRepo         domain.ReadModelRepository
	auditRepo      domain.AuditLogRepository
	eventPublisher domain.EventPublisher
}

func NewInvoiceService(
	invoiceRepo domain.InvoiceRepository,
	rmRepo domain.ReadModelRepository,
	auditRepo domain.AuditLogRepository,
	eventPublisher domain.EventPublisher,
) *InvoiceService {
	return &InvoiceService{
		invoiceRepo:    invoiceRepo,
		rmRepo:         rmRepo,
		auditRepo:      auditRepo,
		eventPublisher: eventPublisher,
	}
}

func (s *InvoiceService) CreateInvoice(ctx context.Context, orgID uuid.UUID, req dto.CreateInvoiceRequest) (*dto.InvoiceResponse, error) {
	// 1. Generate Invoice Number
	invNum, err := s.invoiceRepo.GetNextInvoiceNumber(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate invoice number: %w", err)
	}

	invoiceID := uuid.New()
	invoice := &domain.Invoice{
		ID:             invoiceID,
		OrganizationID: orgID,
		CustomerID:     req.CustomerID,

		ContactID:       req.ContactID,
		OwnerID:         req.OwnerID,
		Subject:         req.Subject,
		InvoiceNumber:   invNum,
		ReferenceNo:     req.ReferenceNo,
		InvoiceDate:     req.InvoiceDate,
		DueDate:         req.DueDate,
		Status:          domain.InvoiceStatusDraft,
		Currency:        req.Currency,
		Adjustment:      req.Adjustment,
		ExciseDuty:      req.ExciseDuty,
		SalesCommission: req.SalesCommission,
		SalesOrder:      req.SalesOrder,
		PurchaseOrder:   req.PurchaseOrder,
		Terms:           req.Terms,
		Notes:           req.Notes,
		BillingStreet:   req.BillingStreet,
		BillingCity:     req.BillingCity,
		BillingState:    req.BillingState,
		BillingCode:     req.BillingCode,
		BillingCountry:  req.BillingCountry,
		ShippingStreet:  req.ShippingStreet,
		ShippingCity:    req.ShippingCity,
		ShippingState:   req.ShippingState,
		ShippingCode:    req.ShippingCode,
		ShippingCountry: req.ShippingCountry,
	}

	var subTotal, discountTotal, taxTotal float64
	items := make([]domain.InvoiceItem, 0, len(req.Items))

	for _, itemReq := range req.Items {
		// Validate item exists in Read Model
		itemName := itemReq.Name
		itemRM, err := s.rmRepo.GetItem(ctx, itemReq.ItemID)
		if err != nil {
			// Fallback: If item not found in read model (e.g. sync lag),
			// use the name provided in the request if available.
			if itemName == "" {
				return nil, fmt.Errorf("item %s not found and no name provided: %w", itemReq.ItemID, err)
			}
			// Log warning here ideally
		} else {
			itemName = itemRM.Name
		}

		itemTotal := (itemReq.Quantity * itemReq.UnitPrice) - itemReq.Discount + itemReq.Tax

		items = append(items, domain.InvoiceItem{
			ID:          uuid.New(),
			InvoiceID:   invoiceID,
			ItemID:      itemReq.ItemID,
			ItemType:    itemReq.ItemType, // Optional
			Name:        itemName,
			Description: itemReq.Description,
			Quantity:    itemReq.Quantity,
			UnitPrice:   itemReq.UnitPrice,
			Discount:    itemReq.Discount,
			Tax:         itemReq.Tax,
			Total:       itemTotal,
		})

		subTotal += (itemReq.Quantity * itemReq.UnitPrice)
		discountTotal += itemReq.Discount
		taxTotal += itemReq.Tax
	}

	invoice.Items = items
	invoice.SubTotal = subTotal
	invoice.DiscountTotal = discountTotal
	invoice.TaxTotal = taxTotal
	invoice.TotalAmount = subTotal - discountTotal + taxTotal + req.Adjustment + req.ExciseDuty
	invoice.BalanceAmount = invoice.TotalAmount

	if err := s.invoiceRepo.Create(ctx, invoice); err != nil {
		return nil, err
	}

	// 2. Publish Event
	s.publishInvoiceCreated(invoice)

	return s.mapToResponse(ctx, invoice), nil
}

func (s *InvoiceService) UpdateInvoice(ctx context.Context, id uuid.UUID, req dto.CreateInvoiceRequest) (*dto.InvoiceResponse, error) {
	// 1. Get Existing Invoice
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if invoice == nil {
		return nil, fmt.Errorf("invoice not found")
	}

	// 2. Update Fields
	invoice.Subject = req.Subject
	// invoice.CustomerID = req.CustomerID // Usually changing customer is restricted, or complicated. Allow for now.
	if req.ContactID != nil {
		invoice.ContactID = req.ContactID
	}
	invoice.InvoiceDate = req.InvoiceDate
	invoice.DueDate = req.DueDate
	invoice.Currency = req.Currency
	invoice.Adjustment = req.Adjustment
	invoice.ExciseDuty = req.ExciseDuty
	invoice.SalesCommission = req.SalesCommission
	invoice.SalesOrder = req.SalesOrder
	invoice.PurchaseOrder = req.PurchaseOrder
	invoice.Terms = req.Terms
	if req.OwnerID != nil {
		invoice.OwnerID = req.OwnerID
	}
	invoice.Notes = req.Notes
	invoice.BillingStreet = req.BillingStreet
	invoice.BillingCity = req.BillingCity
	invoice.BillingState = req.BillingState
	invoice.BillingCode = req.BillingCode
	invoice.BillingCountry = req.BillingCountry
	invoice.ShippingStreet = req.ShippingStreet
	invoice.ShippingCity = req.ShippingCity
	invoice.ShippingState = req.ShippingState
	invoice.ShippingCode = req.ShippingCode
	invoice.ShippingCountry = req.ShippingCountry

	// 3. Update Items
	// Clear existing items
	if err := s.invoiceRepo.ClearItems(ctx, invoice.ID); err != nil {
		return nil, fmt.Errorf("failed to clear existing items: %w", err)
	}

	var subTotal, discountTotal, taxTotal float64
	items := make([]domain.InvoiceItem, 0, len(req.Items))

	for _, itemReq := range req.Items {
		itemName := itemReq.Name
		itemRM, err := s.rmRepo.GetItem(ctx, itemReq.ItemID)
		if err != nil {
			if itemName == "" {
				return nil, fmt.Errorf("item %s not found and no name provided: %w", itemReq.ItemID, err)
			}
		} else {
			itemName = itemRM.Name
		}

		itemTotal := (itemReq.Quantity * itemReq.UnitPrice) - itemReq.Discount + itemReq.Tax

		items = append(items, domain.InvoiceItem{
			ID:          uuid.New(),
			InvoiceID:   invoice.ID,
			ItemID:      itemReq.ItemID,
			ItemType:    itemReq.ItemType,
			Name:        itemName,
			Description: itemReq.Description,
			Quantity:    itemReq.Quantity,
			UnitPrice:   itemReq.UnitPrice,
			Discount:    itemReq.Discount,
			Tax:         itemReq.Tax,
			Total:       itemTotal,
		})

		subTotal += (itemReq.Quantity * itemReq.UnitPrice)
		discountTotal += itemReq.Discount
		taxTotal += itemReq.Tax
	}

	invoice.Items = items
	invoice.SubTotal = subTotal
	invoice.DiscountTotal = discountTotal
	invoice.TaxTotal = taxTotal
	invoice.TotalAmount = subTotal - discountTotal + taxTotal + req.Adjustment + req.ExciseDuty
	invoice.BalanceAmount = invoice.TotalAmount // Assuming no payments yet, or simple Recalc. For partial, we need to subtract payments.

	// Recalculate Balance with existing payments (if any)
	// Currently GetByID preloads Payments.
	paidAmount := 0.0
	for _, p := range invoice.Payments {
		paidAmount += p.Amount
	}
	invoice.PaidAmount = paidAmount
	invoice.BalanceAmount = invoice.TotalAmount - paidAmount

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return nil, err
	}

	// 4. Publish Event (?) - InvoiceUpdated
	// s.publishInvoiceUpdated(invoice)

	return s.mapToResponse(ctx, invoice), nil
}

func (s *InvoiceService) publishInvoiceCreated(inv *domain.Invoice) {
	payload := shared_events.InvoiceCreatedPayload{
		InvoiceID:      inv.ID.String(),
		OrganizationID: inv.OrganizationID.String(),
		CustomerID:     inv.CustomerID.String(),
		Subject:        inv.Subject,
		InvoiceNumber:  inv.InvoiceNumber,
		InvoiceDate:    inv.InvoiceDate.Format(time.RFC3339),
		DueDate:        inv.DueDate.Format(time.RFC3339),
		Status:         string(inv.Status),
		TotalAmount:    inv.TotalAmount,
		Currency:       inv.Currency,
	}

	metadata := shared_events.NewEventMetadata(shared_events.InvoiceCreated, shared_events.AggregateInvoice, inv.ID.String())
	s.eventPublisher.Publish(context.Background(), metadata, payload)
}

func (s *InvoiceService) GetInvoice(ctx context.Context, id uuid.UUID) (*dto.InvoiceResponse, error) {
	inv, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return s.mapToResponse(ctx, inv), nil
}

func (s *InvoiceService) DeleteInvoice(ctx context.Context, id uuid.UUID) error {
	// 1. Get existing invoice to verify it exists
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}

	// 2. Delete the invoice (cascade will handle items)
	if err := s.invoiceRepo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete invoice: %w", err)
	}

	// 3. Optionally publish InvoiceDeleted event
	// s.publishInvoiceDeleted(invoice)

	return nil
}

func (s *InvoiceService) ListInvoices(ctx context.Context, orgID uuid.UUID) ([]dto.InvoiceResponse, error) {
	invoices, err := s.invoiceRepo.List(ctx, map[string]interface{}{"organization_id": orgID})
	if err != nil {
		return nil, err
	}

	res := make([]dto.InvoiceResponse, 0, len(invoices))
	for _, inv := range invoices {
		res = append(res, *s.mapToResponse(ctx, &inv))
	}
	return res, nil
}

func (s *InvoiceService) mapToResponse(ctx context.Context, inv *domain.Invoice) *dto.InvoiceResponse {
	res := &dto.InvoiceResponse{
		ID:              inv.ID,
		InvoiceNumber:   inv.InvoiceNumber,
		Subject:         inv.Subject,
		Status:          string(inv.Status),
		SubTotal:        inv.SubTotal,
		DiscountTotal:   inv.DiscountTotal,
		TaxTotal:        inv.TaxTotal,
		TotalAmount:     inv.TotalAmount,
		PaidAmount:      inv.PaidAmount,
		BalanceAmount:   inv.BalanceAmount,
		Adjustment:      inv.Adjustment,
		ExciseDuty:      inv.ExciseDuty,
		SalesCommission: inv.SalesCommission,
		SalesOrder:      inv.SalesOrder,
		PurchaseOrder:   inv.PurchaseOrder,
		OwnerID:         inv.OwnerID,
		CustomerID:      inv.CustomerID,
		ContactID:       inv.ContactID,
		InvoiceDate:     inv.InvoiceDate,
		DueDate:         inv.DueDate,
		BillingStreet:   inv.BillingStreet,
		BillingCity:     inv.BillingCity,
		BillingState:    inv.BillingState,
		BillingCode:     inv.BillingCode,
		BillingCountry:  inv.BillingCountry,
		ShippingStreet:  inv.ShippingStreet,
		ShippingCity:    inv.ShippingCity,
		ShippingState:   inv.ShippingState,
		ShippingCode:    inv.ShippingCode,
		ShippingCountry: inv.ShippingCountry,
		Notes:           inv.Notes,
		Terms:           inv.Terms,
	}

	// Fetch Customer details from Read Model
	if customer, err := s.rmRepo.GetCustomer(ctx, inv.CustomerID); err == nil && customer != nil {
		res.Customer = &dto.CustomerResponse{
			ID:          customer.ID,
			DisplayName: customer.DisplayName,
			CompanyName: customer.CompanyName,
		}
	}

	// Fetch Contact details from Read Model
	if inv.ContactID != nil {
		if contact, err := s.rmRepo.GetContact(ctx, *inv.ContactID); err == nil && contact != nil {
			res.Contact = &dto.ContactResponse{
				ID:        contact.ID,
				FirstName: contact.FirstName,
				LastName:  contact.LastName,
				Email:     contact.Email,
			}
		}
	}

	if len(inv.Items) > 0 {
		res.Items = make([]dto.ItemResponse, 0, len(inv.Items))
		for _, item := range inv.Items {
			res.Items = append(res.Items, dto.ItemResponse{
				ItemID:      item.ItemID,
				ItemType:    item.ItemType,
				Name:        item.Name,
				Description: item.Description,
				Quantity:    item.Quantity,
				UnitPrice:   item.UnitPrice,
				Discount:    item.Discount,
				Tax:         item.Tax,
				Total:       item.Total,
			})
		}
	}

	return res
}

func (s *InvoiceService) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus domain.InvoiceStatus, notes string, performedBy string) error {
	invoice, err := s.invoiceRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}

	oldStatus := string(invoice.Status)
	invoice.Status = newStatus

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return err
	}

	// Record Audit Log
	auditLog := &domain.InvoiceAuditLog{
		ID:             uuid.New(),
		OrganizationID: invoice.OrganizationID,
		InvoiceID:      invoice.ID,
		Action:         "status_change",
		OldStatus:      oldStatus,
		NewStatus:      string(newStatus),
		Notes:          notes,
		PerformedBy:    performedBy,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.auditRepo.Create(ctx, auditLog); err != nil {
		// Log error but don't fail the operation
		fmt.Printf("failed to create audit log: %v\n", err)
	}

	return nil
}

func (s *InvoiceService) GetAuditLogs(ctx context.Context, invoiceID uuid.UUID) ([]domain.InvoiceAuditLog, error) {
	return s.auditRepo.ListByInvoiceID(ctx, invoiceID)
}
