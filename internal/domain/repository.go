package domain

import (
	"context"

	"github.com/google/uuid"
)

type InvoiceRepository interface {
	Create(ctx context.Context, invoice *Invoice) error
	Update(ctx context.Context, invoice *Invoice) error
	GetByID(ctx context.Context, id uuid.UUID) (*Invoice, error)
	List(ctx context.Context, filter map[string]interface{}) ([]Invoice, error)
	ListByModule(ctx context.Context, orgID uuid.UUID, sourceSystem SourceSystem) ([]Invoice, error)
	Delete(ctx context.Context, id uuid.UUID) error
	GetNextInvoiceNumber(ctx context.Context, orgID uuid.UUID) (string, error)
	ClearItems(ctx context.Context, invoiceID uuid.UUID) error
}

type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	Update(ctx context.Context, payment *Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*Payment, error)
	GetByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]Payment, error)
	ListByInvoice(ctx context.Context, invoiceID uuid.UUID) ([]Payment, error)
	List(ctx context.Context) ([]Payment, error)
	ListByModule(ctx context.Context, orgID uuid.UUID, sourceSystem SourceSystem) ([]Payment, error)
}

type ReadModelRepository interface {
	GetCustomer(ctx context.Context, id uuid.UUID) (*CustomerRM, error)
	SearchCustomers(ctx context.Context, orgID uuid.UUID, query string) ([]CustomerRM, error)
	GetItem(ctx context.Context, id uuid.UUID) (*ItemRM, error)
	SearchItems(ctx context.Context, orgID uuid.UUID, query string) ([]ItemRM, error)
	GetContact(ctx context.Context, id uuid.UUID) (*ContactRM, error)
	GetPrimaryContact(ctx context.Context, customerID uuid.UUID) (*ContactRM, error)
	SearchContacts(ctx context.Context, orgID uuid.UUID, customerID uuid.UUID, query string) ([]ContactRM, error)
}

type AuditLogRepository interface {
	Create(ctx context.Context, log *InvoiceAuditLog) error
	ListByInvoiceID(ctx context.Context, invoiceID uuid.UUID) ([]InvoiceAuditLog, error)
}
