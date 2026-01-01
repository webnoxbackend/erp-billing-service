package application

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/domain"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
)

type PaymentService struct {
	paymentRepo    domain.PaymentRepository
	invoiceRepo    domain.InvoiceRepository
	eventPublisher domain.EventPublisher
}

func NewPaymentService(
	paymentRepo domain.PaymentRepository,
	invoiceRepo domain.InvoiceRepository,
	eventPublisher domain.EventPublisher,
) *PaymentService {
	return &PaymentService{
		paymentRepo:    paymentRepo,
		invoiceRepo:    invoiceRepo,
		eventPublisher: eventPublisher,
	}
}

func (s *PaymentService) RecordPayment(ctx context.Context, orgID uuid.UUID, invoiceID uuid.UUID, amount float64, method string, ref string) (*domain.Payment, error) {
	// 1. Get Invoice
	invoice, err := s.invoiceRepo.GetByID(ctx, invoiceID)
	if err != nil {
		return nil, fmt.Errorf("invoice not found: %w", err)
	}

	// 2. Create Payment Record
	payment := &domain.Payment{
		ID:             uuid.New(),
		OrganizationID: orgID,
		InvoiceID:      invoiceID,
		Amount:         amount,
		PaymentDate:    time.Now().UTC(),
		PaymentMethod:  method,
		TransactionRef: ref,
	}

	if err := s.paymentRepo.Create(ctx, payment); err != nil {
		return nil, err
	}

	// 3. Update Invoice Status
	invoice.PaidAmount += amount
	invoice.BalanceAmount = invoice.TotalAmount - invoice.PaidAmount

	if invoice.BalanceAmount <= 0 {
		invoice.Status = domain.InvoiceStatusPaid
		invoice.BalanceAmount = 0
	} else if invoice.PaidAmount > 0 {
		invoice.Status = domain.InvoiceStatusPartial
	}

	if err := s.invoiceRepo.Update(ctx, invoice); err != nil {
		return nil, err
	}

	// 4. Publish Event
	s.publishPaymentCreated(payment)

	return payment, nil
}

func (s *PaymentService) publishPaymentCreated(p *domain.Payment) {
	payload := shared_events.PaymentCreatedPayload{
		PaymentID:      p.ID.String(),
		OrganizationID: p.OrganizationID.String(),
		InvoiceID:      p.InvoiceID.String(),
		Amount:         p.Amount,
		PaymentDate:    p.PaymentDate,
		PaymentMethod:  p.PaymentMethod,
		ReferenceNo:    p.TransactionRef,
		Status:         "completed",
	}

	metadata := shared_events.NewEventMetadata(shared_events.PaymentCreated, shared_events.AggregatePayment, p.ID.String())
	s.eventPublisher.Publish(context.Background(), metadata, payload)
}
