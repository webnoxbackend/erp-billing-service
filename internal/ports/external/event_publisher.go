package external

import "erp-billing-service/internal/domain"

// EventPublisher defines the interface for publishing domain events
type EventPublisher interface {
	// Publish publishes a domain event
	Publish(event *domain.Event) error
}

