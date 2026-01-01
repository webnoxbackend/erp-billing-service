package kafka

import (
	"context"
	"fmt"
	"log"

	"erp-billing-service/internal/domain"

	shared_events "github.com/efs/shared-events"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type EventHandler struct {
	db *gorm.DB
}

func NewEventHandler(db *gorm.DB) *EventHandler {
	return &EventHandler{db: db}
}

func (h *EventHandler) HandleMessage(ctx context.Context, topic string, key string, value []byte, headers map[string]string) error {
	return h.Handle(ctx, value)
}

func (h *EventHandler) Handle(ctx context.Context, data []byte) error {
	baseEvent, err := shared_events.Unmarshal(data)
	if err != nil {
		return fmt.Errorf("failed to unmarshal event: %w", err)
	}

	log.Printf("Processing event: %s for aggregate: %s (%s)",
		baseEvent.Metadata.EventType,
		baseEvent.Metadata.AggregateType,
		baseEvent.Metadata.AggregateID)

	return h.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		switch baseEvent.Metadata.AggregateType {
		case shared_events.AggregateCustomer:
			return h.handleCustomerEvent(tx, baseEvent)
		case shared_events.AggregateContact:
			return h.handleContactEvent(tx, baseEvent)
		case shared_events.AggregateService:
			return h.handleServiceEvent(tx, baseEvent)
		case shared_events.AggregatePart:
			return h.handlePartEvent(tx, baseEvent)
		default:
			log.Printf("Ignoring unrelated aggregate type: %s", baseEvent.Metadata.AggregateType)
			return nil
		}
	})
}

func (h *EventHandler) handleCustomerEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	var payload shared_events.CustomerCreatedPayload
	if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
		return err
	}

	customerID, _ := uuid.Parse(payload.CustomerID)
	orgID, _ := uuid.Parse(payload.OrganizationID)

	rm := domain.CustomerRM{
		ID:             customerID,
		OrganizationID: orgID,
		DisplayName:    fmt.Sprintf("%s %s", payload.FirstName, payload.LastName),
		CompanyName:    payload.CompanyName,
		Email:          payload.Email,
		Phone:          payload.Phone,
		UpdatedAt:      event.Metadata.OccurredAt,
	}

	return tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&rm).Error
}

func (h *EventHandler) handleContactEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	var payload shared_events.ContactCreatedPayload
	if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
		return err
	}

	contactID, _ := uuid.Parse(payload.ContactID)
	orgID, _ := uuid.Parse(payload.OrganizationID)
	var customerID uuid.UUID
	if payload.CompanyID != nil {
		customerID, _ = uuid.Parse(*payload.CompanyID)
	}

	rm := domain.ContactRM{
		ID:             contactID,
		OrganizationID: orgID,
		CustomerID:     customerID,
		FirstName:      payload.FirstName,
		LastName:       payload.LastName,
		Email:          payload.Email,
		Phone:          payload.Phone,
		UpdatedAt:      event.Metadata.OccurredAt,
	}

	return tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&rm).Error
}

func (h *EventHandler) handleServiceEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	var payload shared_events.ServiceCreatedPayload
	if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
		return err
	}

	serviceID, _ := uuid.Parse(payload.ServiceID)
	orgID, _ := uuid.Parse(payload.OrganizationID)

	rm := domain.ItemRM{
		ID:             serviceID,
		OrganizationID: orgID,
		Name:           payload.Name,
		Description:    payload.Description,
		ItemType:       "service",
		Price:          payload.BasePrice,
		UpdatedAt:      event.Metadata.OccurredAt,
	}

	return tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&rm).Error
}

func (h *EventHandler) handlePartEvent(tx *gorm.DB, event *shared_events.BaseEvent) error {
	var payload shared_events.PartCreatedPayload
	if err := shared_events.UnmarshalPayload(event, &payload); err != nil {
		return err
	}

	partID, _ := uuid.Parse(payload.PartID)
	orgID, _ := uuid.Parse(payload.OrganizationID)

	rm := domain.ItemRM{
		ID:             partID,
		OrganizationID: orgID,
		Name:           payload.Name,
		Description:    payload.Description,
		ItemType:       "part",
		Price:          payload.UnitPrice,
		SKU:            payload.PartNumber,
		UpdatedAt:      event.Metadata.OccurredAt,
	}

	return tx.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&rm).Error
}
