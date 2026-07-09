package application

import (
	"context"
	"fmt"
	"log"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/outbound"
	"erp-billing-service/internal/ports/repositories"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
)

// PaymentService handles payment-related business logic
type PaymentService struct {
	paymentRepo    domain.PaymentRepository
	invoiceRepo    domain.InvoiceRepository
	salesOrderRepo repositories.SalesOrderRepository
	rmRepo         domain.ReadModelRepository
	auditRepo      domain.AuditLogRepository
	eventPublisher domain.EventPublisher
	customerClient outbound.CustomerClient
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	paymentRepo domain.PaymentRepository,
	invoiceRepo domain.InvoiceRepository,
	salesOrderRepo repositories.SalesOrderRepository,
	rmRepo domain.ReadModelRepository,
	auditRepo domain.AuditLogRepository,
	eventPublisher domain.EventPublisher,
	customerClient outbound.CustomerClient,
) *PaymentService {
	return &PaymentService{
		paymentRepo:    paymentRepo,
		invoiceRepo:    invoiceRepo,
		salesOrderRepo: salesOrderRepo,
		rmRepo:         rmRepo,
		auditRepo:      auditRepo,
		eventPublisher: eventPublisher,
		customerClient: customerClient,
	}
}

// RecordPayment records a new payment against an invoice
func (s *PaymentService) RecordPayment(ctx context.Context, req dto.RecordPaymentRequest) (*dto.PaymentResponse, error) {
	// 1. Get invoice and validate
	invoice, err := s.invoiceRepo.GetByID(ctx, req.InvoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get invoice: %w", err)
	}
	if invoice == nil {
		return nil, fmt.Errorf("invoice not found")
	}

	// 2. Validate invoice can receive payment
	if !invoice.CanReceivePayment() {
		return nil, fmt.Errorf("invoice in %s status cannot receive payments - only SENT invoices can be paid", invoice.Status)
	}

	// 3. Validate payment amount
	if req.Amount <= 0 {
		return nil, fmt.Errorf("payment amount must be greater than zero")
	}

	const epsilon = 0.01
	if req.Amount > (invoice.BalanceAmount + epsilon) {
		return nil, fmt.Errorf("payment amount (%.2f) exceeds balance due (%.2f)", req.Amount, invoice.BalanceAmount)
	}

	// 3.5 Parse payment date
	paymentDate, err := time.Parse(time.RFC3339, req.PaymentDate)
	if err != nil {
		// Fallback for YYYY-MM-DD
		paymentDate, err = time.Parse("2006-01-02", req.PaymentDate)
		if err != nil {
			return nil, fmt.Errorf("invalid payment date format: %w", err)
		}
	}

	// 4. Create payment and update invoice in a single save flow
	fmt.Printf("[INFO] Recording payment of %.2f for invoice %s (Current Balance: %.2f)\n", req.Amount, invoice.ID, invoice.BalanceAmount)

	payment := &domain.Payment{
		ID:             uuid.New(),
		OrganizationID: invoice.OrganizationID,
		InvoiceID:      invoice.ID,
		Amount:         req.Amount,
		PaymentDate:    paymentDate,
		Method:         domain.PaymentMethod(req.Method),
		Reference:      req.Reference,
		Status:         domain.PaymentStatusCompleted,
		Notes:          req.Notes,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, fmt.Errorf("failed to create payment: %w", err)
	}

	// 5. Update invoice amounts
	oldStatus := invoice.Status
	invoice.PaidAmount += req.Amount
	invoice.BalanceAmount -= req.Amount

	// 6. Derive new status from payment amounts
	invoice.Status = invoice.CalculateStatus()
	fmt.Printf("[INFO] Invoice %s status updated: %s -> %s (Remaining Balance: %.2f)\n", invoice.ID, oldStatus, invoice.Status, invoice.BalanceAmount)

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return nil, fmt.Errorf("failed to update invoice: %w", err)
	}

	// 7. Create audit log
	auditLog := &domain.InvoiceAuditLog{
		ID:             uuid.New(),
		OrganizationID: invoice.OrganizationID,
		InvoiceID:      invoice.ID,
		Action:         "PAYMENT_RECORDED",
		OldStatus:      string(oldStatus),
		NewStatus:      string(invoice.Status),
		Notes:          fmt.Sprintf("Payment of %.2f recorded via %s", req.Amount, req.Method),
		PerformedBy:    "system", // TODO: Get from context
		CreatedAt:      time.Now().UTC(),
	}
	s.auditRepo.Create(ctx, auditLog)

	// 8. Emit events
	s.publishPaymentReceived(payment, invoice)
	s.publishPaymentRecorded(payment, invoice) // For work order status updates

	// Only publish invoice paid event when fully paid
	if invoice.Status == domain.InvoiceStatusPaid {
		s.publishInvoicePaid(invoice)

		// Direct update for Sales Order if linked (Inventory/Internal)
		if invoice.SalesOrderID != nil {
			// We update status to paid directly
			s.salesOrderRepo.UpdateStatus(ctx, *invoice.SalesOrderID, domain.SalesOrderStatusPaid)
		}
	}

	return s.mapToResponse(ctx, payment), nil
}

// ListAllPayments returns all payments
func (s *PaymentService) ListAllPayments(ctx context.Context) ([]*dto.PaymentResponse, error) {
	payments, err := s.paymentRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}

	responses := make([]*dto.PaymentResponse, len(payments))
	for i, payment := range payments {
		responses[i] = s.mapToResponse(ctx, &payment)
	}

	return responses, nil
}

// ListPaymentsByModule returns payments filtered by invoice source_system
func (s *PaymentService) ListPaymentsByModule(ctx context.Context, orgID uuid.UUID, sourceSystem domain.SourceSystem) ([]*dto.PaymentResponse, error) {
	payments, err := s.paymentRepo.ListByModule(ctx, orgID, sourceSystem)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments by module: %w", err)
	}

	responses := make([]*dto.PaymentResponse, len(payments))
	for i, payment := range payments {
		responses[i] = s.mapToResponse(ctx, &payment)
	}

	return responses, nil
}

// VoidPayment voids an existing payment
func (s *PaymentService) VoidPayment(ctx context.Context, paymentID uuid.UUID, notes string) error {
	// 1. Get payment
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return fmt.Errorf("failed to get payment: %w", err)
	}
	if payment == nil {
		return fmt.Errorf("payment not found")
	}

	// 2. Validate can void
	if !payment.CanVoid() {
		return fmt.Errorf("payment in %s status cannot be voided", payment.Status)
	}

	// 3. Get invoice
	invoice, err := s.invoiceRepo.GetByID(ctx, payment.InvoiceID)
	if err != nil {
		return fmt.Errorf("failed to get invoice: %w", err)
	}

	// 4. Update payment status
	payment.Status = domain.PaymentStatusVoid
	payment.Notes = fmt.Sprintf("%s\n[VOIDED: %s]", payment.Notes, notes)
	payment.UpdatedAt = time.Now().UTC()

	if err := s.paymentRepo.Update(ctx, payment); err != nil {
		return fmt.Errorf("failed to void payment: %w", err)
	}

	// 5. Recalculate invoice amounts
	oldInvoiceStatus := invoice.Status
	invoice.PaidAmount -= payment.Amount
	invoice.BalanceAmount += payment.Amount

	// 6. Derive new status
	invoice.Status = invoice.CalculateStatus()

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	// 7. Create audit log
	auditLog := &domain.InvoiceAuditLog{
		ID:             uuid.New(),
		OrganizationID: invoice.OrganizationID,
		InvoiceID:      invoice.ID,
		Action:         "PAYMENT_VOIDED",
		OldStatus:      string(oldInvoiceStatus),
		NewStatus:      string(invoice.Status),
		Notes:          fmt.Sprintf("Payment of %.2f voided. Reason: %s", payment.Amount, notes),
		PerformedBy:    "system", // TODO: Get from context
		CreatedAt:      time.Now().UTC(),
	}
	s.auditRepo.Create(ctx, auditLog)

	return nil
}

// ListPaymentsByInvoice returns all payments for an invoice
func (s *PaymentService) ListPaymentsByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]dto.PaymentResponse, error) {
	payments, err := s.paymentRepo.ListByInvoice(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}

	responses := make([]dto.PaymentResponse, len(payments))
	for i, payment := range payments {
		responses[i] = *s.mapToResponse(ctx, &payment)
	}

	return responses, nil
}

// ListPaymentsByCustomerEmail lists all payments and outstanding invoices for a customer email, optionally filtered by status
func (s *PaymentService) ListPaymentsByCustomerEmail(ctx context.Context, orgID uuid.UUID, customerEmail string, statusFilter string) (*dto.CustomerPaymentHistoryResponse, error) {
	customerIDs, err := s.rmRepo.GetCustomerIDsByEmailAndOrg(ctx, orgID, customerEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup customer IDs by email: %w", err)
	}
	if len(customerIDs) == 0 {
		return &dto.CustomerPaymentHistoryResponse{
			Completed:     []*dto.PaymentResponse{},
			PartiallyPaid: []dto.InvoiceResponse{},
			Unpaid:        []dto.InvoiceResponse{},
		}, nil
	}

	completedResponses := make([]*dto.PaymentResponse, 0)
	partiallyPaidInvoices := make([]dto.InvoiceResponse, 0)
	unpaidInvoices := make([]dto.InvoiceResponse, 0)

	// Determine what we need to fetch based on statusFilter
	fetchPayments := statusFilter == "" || statusFilter == "completed"
	fetchInvoices := statusFilter == "" || statusFilter == "partially_paid" || statusFilter == "unpaid"

	// 1. Fetch completed payments if required
	if fetchPayments {
		payments, err := s.paymentRepo.ListByCustomerIDs(ctx, orgID, customerIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to list customer payments: %w", err)
		}

		for _, payment := range payments {
			if payment.Status == domain.PaymentStatusCompleted {
				completedResponses = append(completedResponses, s.mapToResponse(ctx, &payment))
			}
		}
	}

	// 2. Fetch customer invoices if required
	if fetchInvoices {
		invoiceFilter := map[string]interface{}{
			"organization_id": orgID,
			"customer_id":     customerIDs,
		}
		invoices, err := s.invoiceRepo.List(ctx, invoiceFilter)
		if err != nil {
			return nil, fmt.Errorf("failed to list customer invoices: %w", err)
		}

		for _, inv := range invoices {
			switch inv.Status {
			case domain.InvoiceStatusPartial:
				if statusFilter == "" || statusFilter == "partially_paid" {
					partiallyPaidInvoices = append(partiallyPaidInvoices, s.mapInvoiceToResponse(ctx, &inv))
				}
			case domain.InvoiceStatusSent, domain.InvoiceStatusOverdue:
				if statusFilter == "" || statusFilter == "unpaid" {
					unpaidInvoices = append(unpaidInvoices, s.mapInvoiceToResponse(ctx, &inv))
				}
			}
		}
	}

	return &dto.CustomerPaymentHistoryResponse{
		Completed:     completedResponses,
		PartiallyPaid: partiallyPaidInvoices,
		Unpaid:        unpaidInvoices,
	}, nil
}

func (s *PaymentService) mapInvoiceToResponse(ctx context.Context, inv *domain.Invoice) dto.InvoiceResponse {
	res := dto.InvoiceResponse{
		ID:            inv.ID,
		InvoiceNumber: inv.InvoiceNumber,
		SourceSystem:      string(inv.SourceSystem),
		SourceReferenceID: inv.SourceReferenceID,
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
		PaymentTerms:    inv.PaymentTerms,
		Currency:        inv.Currency,
		ServiceCategoryID: inv.ServiceCategoryID,
		OwnerID:         inv.OwnerID,
		CustomerID:        inv.CustomerID,
		ContactID:         inv.ContactID,
		InvoiceDate:       ConvertToOrgTZValue(ctx, inv.InvoiceDate, inv.OrganizationID, s.rmRepo),
		DueDate:         ConvertToOrgTZValue(ctx, inv.DueDate, inv.OrganizationID, s.rmRepo),
		CreatedAt:       ConvertToOrgTZValue(ctx, inv.CreatedAt, inv.OrganizationID, s.rmRepo),
		UpdatedAt:       ConvertToOrgTZValue(ctx, inv.UpdatedAt, inv.OrganizationID, s.rmRepo),
		PDFPath: inv.PDFPath,
		BillingAddressID:  inv.BillingAddressID,
		ShippingAddressID: inv.ShippingAddressID,
		ServiceAddressID:  inv.ServiceAddressID,
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

	customer, err := s.rmRepo.GetCustomer(ctx, inv.CustomerID)
	if err == nil && customer != nil {
		res.Customer = &dto.CustomerResponse{
			ID:          customer.ID,
			DisplayName: customer.DisplayName,
			CompanyName: customer.CompanyName,
		}
	} else {
		res.Customer = &dto.CustomerResponse{
			ID:          inv.CustomerID,
			DisplayName: "Unknown Customer",
			CompanyName: "Unknown Customer",
		}
	}

	var contact *domain.ContactRM
	if inv.ContactID != nil {
		contact, _ = s.rmRepo.GetContact(ctx, *inv.ContactID)
	} else {
		contact, _ = s.rmRepo.GetPrimaryContact(ctx, inv.CustomerID)
	}

	if contact != nil {
		res.Contact = &dto.ContactResponse{
			ID:        contact.ID,
			FirstName: contact.FirstName,
			LastName:  contact.LastName,
			Email:     contact.Email,
		}
	}

	if len(inv.Items) > 0 {
		res.Items = make([]dto.ItemResponse, 0, len(inv.Items))
		for _, item := range inv.Items {
			itemResp := dto.ItemResponse{
				ItemID:            item.ItemID,
				ItemType:          item.ItemType,
				Name:              item.Name,
				Description:       item.Description,
				Quantity:          item.Quantity,
				UnitPrice:         item.UnitPrice,
				Discount:          item.Discount,
				Tax:               item.Tax,
				Total:             item.Total,
				ServiceCategoryID: item.ServiceCategoryID,
			}
			res.Items = append(res.Items, itemResp)
		}
	}

	return res
}


// GetPayment returns a single payment by ID
func (s *PaymentService) GetPayment(ctx context.Context, paymentID uuid.UUID) (*dto.PaymentResponse, error) {
	payment, err := s.paymentRepo.GetByID(ctx, paymentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment: %w", err)
	}
	if payment == nil {
		return nil, fmt.Errorf("payment not found")
	}

	return s.mapToResponse(ctx, payment), nil
}

// Helper methods

func (s *PaymentService) getCustomer(ctx context.Context, id uuid.UUID) (*domain.CustomerRM, error) {
	customer, err := s.rmRepo.GetCustomer(ctx, id)
	if (err != nil || customer == nil || customer.DisplayName == "") && s.customerClient != nil {
		log.Printf("[INFO] Customer %s missing or invalid in ReadModel, fetching from Customer Service\n", id)
		remoteCustomer, remoteErr := s.customerClient.GetCustomer(ctx, id)
		if remoteErr == nil && remoteCustomer != nil {
			return remoteCustomer, nil
		}
	}
	return customer, err
}

func (s *PaymentService) mapToResponse(ctx context.Context, payment *domain.Payment) *dto.PaymentResponse {
	resp := &dto.PaymentResponse{
		ID:          payment.ID.String(),
		InvoiceID:   payment.InvoiceID.String(),
		Amount:      payment.Amount,
		Method:      string(payment.Method),
		Reference:   payment.Reference,
		PaymentDate: ConvertToOrgTZValue(ctx, payment.PaymentDate, payment.OrganizationID, s.rmRepo),
		Status:      string(payment.Status),
		Notes:       payment.Notes,
		CreatedAt:   ConvertToOrgTZValue(ctx, payment.CreatedAt, payment.OrganizationID, s.rmRepo),
		UpdatedAt:   ConvertToOrgTZValue(ctx, payment.UpdatedAt, payment.OrganizationID, s.rmRepo),
	}

	// Fetch related invoice and customer info to populate InvoiceNumber and CustomerName
	invoice, err := s.invoiceRepo.GetByID(ctx, payment.InvoiceID)
	if err != nil {
		log.Printf("[PaymentService.mapToResponse] Error fetching invoice %s: %v", payment.InvoiceID, err)
	} else if invoice == nil {
		log.Printf("[PaymentService.mapToResponse] Invoice %s is nil", payment.InvoiceID)
	} else {
		if invoice.InvoiceNumber != nil {
			resp.InvoiceNumber = *invoice.InvoiceNumber
		}
		customer, err := s.getCustomer(ctx, invoice.CustomerID)
		if err != nil {
			log.Printf("[PaymentService.mapToResponse] Error fetching customer %s: %v", invoice.CustomerID, err)
		} else if customer == nil {
			log.Printf("[PaymentService.mapToResponse] Customer %s is nil", invoice.CustomerID)
		} else {
			if customer.DisplayName != "" {
				resp.CustomerName = customer.DisplayName
			} else {
				resp.CustomerName = customer.CompanyName
			}
		}
	}

	return resp
}

func (s *PaymentService) publishPaymentReceived(payment *domain.Payment, invoice *domain.Invoice) {
	event := &domain.PaymentReceivedEvent{
		PaymentID:      payment.ID.String(),
		InvoiceID:      payment.InvoiceID.String(),
		OrganizationID: payment.OrganizationID.String(),
		Amount:         payment.Amount,
		Method:         string(payment.Method),
		PaymentDate:    payment.PaymentDate,
		Timestamp:      time.Now().UTC(),
	}

	metadata := shared_events.NewEventMetadata(
		shared_events.PaymentCreated,
		shared_events.AggregatePayment,
		payment.ID.String(),
	)
	s.eventPublisher.Publish(context.Background(), metadata, event)
}

func (s *PaymentService) publishPaymentRecorded(payment *domain.Payment, invoice *domain.Invoice) {
	event := &domain.PaymentRecordedEvent{
		PaymentID:         payment.ID.String(),
		InvoiceID:         payment.InvoiceID.String(),
		OrganizationID:    payment.OrganizationID.String(),
		Amount:            payment.Amount,
		Method:            string(payment.Method),
		PaymentDate:       payment.PaymentDate,
		InvoiceTotal:      invoice.TotalAmount,
		TotalPaid:         invoice.PaidAmount,
		BalanceDue:        invoice.BalanceAmount,
		SourceSystem:      string(invoice.SourceSystem),
		SourceReferenceID: invoice.SourceReferenceID,
		Timestamp:         time.Now().UTC(),
	}

	metadata := shared_events.NewEventMetadata(
		shared_events.PaymentRecorded,
		shared_events.AggregatePayment,
		payment.ID.String(),
	)
	s.eventPublisher.Publish(context.Background(), metadata, event)
}

func (s *PaymentService) publishInvoicePartiallyPaid(invoice *domain.Invoice) {
	event := &domain.InvoicePartiallyPaidEvent{
		InvoiceID:         invoice.ID.String(),
		OrganizationID:    invoice.OrganizationID.String(),
		SourceSystem:      string(invoice.SourceSystem),
		SourceReferenceID: invoice.SourceReferenceID,
		PaidAmount:        invoice.PaidAmount,
		BalanceDue:        invoice.BalanceAmount,
		Timestamp:         time.Now().UTC(),
	}

	metadata := shared_events.NewEventMetadata(
		shared_events.InvoiceStatusChanged,
		shared_events.AggregateInvoice,
		invoice.ID.String(),
	)
	s.eventPublisher.Publish(context.Background(), metadata, event)
}

func (s *PaymentService) publishInvoicePaid(invoice *domain.Invoice) {
	event := &domain.InvoicePaidEvent{
		InvoiceID:         invoice.ID.String(),
		OrganizationID:    invoice.OrganizationID.String(),
		SourceSystem:      string(invoice.SourceSystem),
		SourceReferenceID: invoice.SourceReferenceID,
		PaidAt:            time.Now().UTC(),
		TotalAmount:       invoice.TotalAmount,
		Timestamp:         time.Now().UTC(),
	}

	metadata := shared_events.NewEventMetadata(
		shared_events.InvoiceStatusChanged, // Or InvoicePaid if we add it, but status_changed is safer
		shared_events.AggregateInvoice,
		invoice.ID.String(),
	)
	s.eventPublisher.Publish(context.Background(), metadata, event)
}
