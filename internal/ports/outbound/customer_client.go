package outbound

import (
	"context"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
)

type CustomerClient interface {
	GetCustomer(ctx context.Context, id uuid.UUID) (*domain.CustomerRM, error)
	GetContact(ctx context.Context, id uuid.UUID) (*domain.ContactRM, error)
}
