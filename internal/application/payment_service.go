package application

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/application/dto"
	"erp-billing-service/internal/domain"
	"erp-billing-service/internal/ports/repositories"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
)

// PaymentService handles payment-related business logic
type PaymentService struct {
	paymentRepo    domain.PaymentRepository
	invoiceRepo    domain.InvoiceRepository
	salesOrderRepo repositories.SalesOrderRepository
	auditRepo      domain.AuditLogRepository
	eventPublisher domain.EventPublisher
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	paymentRepo domain.PaymentRepository,
	invoiceRepo domain.InvoiceRepository,
	salesOrderRepo repositories.SalesOrderRepository,
	auditRepo domain.AuditLogRepository,
	eventPublisher domain.EventPublisher,
) *PaymentService {
	return &PaymentService{
		paymentRepo:    paymentRepo,
		invoiceRepo:    invoiceRepo,
		salesOrderRepo: salesOrderRepo,
		auditRepo:      auditRepo,
		eventPublisher: eventPublisher,
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

	return s.mapToResponse(payment), nil
}

// ListAllPayments returns all payments
func (s *PaymentService) ListAllPayments(ctx context.Context) ([]*dto.PaymentResponse, error) {
	payments, err := s.paymentRepo.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list payments: %w", err)
	}

	responses := make([]*dto.PaymentResponse, len(payments))
	for i, payment := range payments {
		responses[i] = s.mapToResponse(&payment)
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
		responses[i] = s.mapToResponse(&payment)
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
		responses[i] = *s.mapToResponse(&payment)
	}

	return responses, nil
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

	return s.mapToResponse(payment), nil
}

// Helper methods

func (s *PaymentService) mapToResponse(payment *domain.Payment) *dto.PaymentResponse {
	return &dto.PaymentResponse{
		ID:          payment.ID.String(),
		InvoiceID:   payment.InvoiceID.String(),
		Amount:      payment.Amount,
		Method:      string(payment.Method),
		Reference:   payment.Reference,
		PaymentDate: payment.PaymentDate,
		Status:      string(payment.Status),
		Notes:       payment.Notes,
		CreatedAt:   payment.CreatedAt,
	}
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
