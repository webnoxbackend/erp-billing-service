package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	sharedEvents "github.com/efs/shared-events"
	shared_kafka "github.com/efs/shared-kafka"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SubscriptionEventHandler handles subscription plan and org subscription events from org-service
type SubscriptionEventHandler struct {
	db *gorm.DB
}

// NewSubscriptionEventHandler creates a new SubscriptionEventHandler
func NewSubscriptionEventHandler(db *gorm.DB) *SubscriptionEventHandler {
	return &SubscriptionEventHandler{db: db}
}

// HandleMessage implements shared_kafka.MessageHandler
func (h *SubscriptionEventHandler) HandleMessage(ctx context.Context, topic string, key string, value []byte, headers map[string]string) error {
	var event struct {
		Metadata sharedEvents.EventMetadata `json:"metadata"`
		Payload  json.RawMessage            `json:"payload"`
	}
	if err := json.Unmarshal(value, &event); err != nil {
		log.Printf("[BillingSubConsumer] Failed to unmarshal event: %v", err)
		return nil // non-blocking: skip bad messages
	}

	log.Printf("[BillingSubConsumer] Received event: %s", event.Metadata.EventType)

	switch event.Metadata.EventType {
	case sharedEvents.SubscriptionPlanCreated, sharedEvents.SubscriptionPlanUpdated:
		return h.handlePlanUpsert(ctx, event.Payload)
	case sharedEvents.SubscriptionPlanDeleted:
		return h.handlePlanDeleted(ctx, event.Payload)
	case sharedEvents.OrganizationSubscribed:
		return h.handleOrgSubscribed(ctx, event.Payload)
	default:
		return nil
	}
}

func (h *SubscriptionEventHandler) handlePlanUpsert(ctx context.Context, raw json.RawMessage) error {
	var payload sharedEvents.SubscriptionPlanUpdatedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[BillingSubConsumer] Failed to unmarshal plan payload: %v", err)
		return nil
	}

	err := h.db.WithContext(ctx).Exec(`
		INSERT INTO plans_readonly (id, name, description, price, valid_days, badge, status, is_per_user, is_active, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), ?)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			price = EXCLUDED.price,
			valid_days = EXCLUDED.valid_days,
			badge = EXCLUDED.badge,
			status = EXCLUDED.status,
			is_per_user = EXCLUDED.is_per_user,
			is_active = EXCLUDED.is_active,
			sort_order = EXCLUDED.sort_order,
			updated_at = EXCLUDED.updated_at
	`, payload.PlanID, payload.Name, payload.Description, payload.Price,
		payload.ValidDays, payload.Badge, payload.Status,
		payload.IsPerUser, payload.IsActive, payload.SortOrder, payload.UpdatedAt,
	).Error
	if err != nil {
		log.Printf("[BillingSubConsumer] Failed to upsert plans_readonly for plan %s: %v", payload.PlanID, err)
		return nil
	}

	for _, r := range payload.Restrictions {
		var deletedAt interface{} = nil
		if r.DeletedAt != nil {
			deletedAt = *r.DeletedAt
		}
		h.db.WithContext(ctx).Exec(`
			INSERT INTO plan_restrictions_readonly (id, plan_id, restriction_key, restriction_value, description, created_at, updated_at, deleted_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				plan_id = EXCLUDED.plan_id,
				restriction_key = EXCLUDED.restriction_key,
				restriction_value = EXCLUDED.restriction_value,
				description = EXCLUDED.description,
				updated_at = EXCLUDED.updated_at,
				deleted_at = EXCLUDED.deleted_at
		`, r.ID, r.PlanID, r.RestrictionKey, r.RestrictionValue, r.Description,
			r.CreatedAt, r.UpdatedAt, deletedAt,
		)
	}
	log.Printf("[BillingSubConsumer] Upserted plan %s (%s) with %d restrictions", payload.PlanID, payload.Name, len(payload.Restrictions))
	return nil
}

func (h *SubscriptionEventHandler) handlePlanDeleted(ctx context.Context, raw json.RawMessage) error {
	var payload sharedEvents.SubscriptionPlanDeletedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[BillingSubConsumer] Failed to unmarshal plan deleted payload: %v", err)
		return nil
	}
	now := time.Now()
	h.db.WithContext(ctx).Exec(`UPDATE plan_restrictions_readonly SET deleted_at = ? WHERE plan_id = ? AND deleted_at IS NULL`, now, payload.PlanID)
	h.db.WithContext(ctx).Exec(`DELETE FROM plans_readonly WHERE id = ?`, payload.PlanID)
	log.Printf("[BillingSubConsumer] Deleted plan %s from readonly tables", payload.PlanID)
	return nil
}

func (h *SubscriptionEventHandler) handleOrgSubscribed(ctx context.Context, raw json.RawMessage) error {
	var payload sharedEvents.OrganizationSubscribedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		log.Printf("[BillingSubConsumer] Failed to unmarshal org subscribed payload: %v", err)
		return nil
	}

	row := map[string]interface{}{
		"id":                       payload.SubscriptionID,
		"organization_id":          payload.OrganizationID,
		"plan_id":                  payload.PlanID,
		"status":                   payload.Status,
		"start_date":               payload.StartDate,
		"expiry_date":              payload.ExpiryDate,
		"selected_module_ids":      payload.SelectedModuleIDs,
		"total_price":              payload.TotalPrice,
		"user_count":               payload.UserCount,
		"notes":                    payload.Notes,
		"billing_period":           payload.BillingPeriod,
		"cancelled_at":             payload.CancelledAt,
		"trial_ends_at":            payload.TrialEndsAt,
		"razorpay_subscription_id": "",
		"payment_method_id":        nil,
		"created_at":               payload.CreatedAt,
		"updated_at":               payload.UpdatedAt,
	}

	err := h.db.WithContext(ctx).Table("organization_subscriptions_readonly").
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"organization_id", "plan_id", "status", "start_date", "expiry_date",
				"selected_module_ids", "total_price", "user_count", "notes",
				"billing_period", "cancelled_at", "trial_ends_at", "updated_at",
			}),
		}).
		Create(row).Error

	if err != nil {
		log.Printf("[BillingSubConsumer] Failed to upsert org subscription %s: %v", payload.SubscriptionID, err)
		return nil
	}
	log.Printf("[BillingSubConsumer] Upserted org subscription %s for org %s (status=%s)",
		payload.SubscriptionID, payload.OrganizationID, payload.Status)
	return nil
}

// StartSubscriptionConsumer starts a dedicated Kafka consumer for subscription/plan topics in billing service
func StartSubscriptionConsumer(kafkaCfg shared_kafka.KafkaConfig, db *gorm.DB) (*shared_kafka.ConsumerGroup, error) {
	handler := NewSubscriptionEventHandler(db)
	topics := []string{
		"org.subscription_plans",
		"org.organization_subscriptions",
	}
	groupID := "billing-subscription-consumer"

	consumer, err := shared_kafka.NewConsumerGroup(kafkaCfg, groupID, topics, handler, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create billing subscription consumer: %w", err)
	}
	consumer.Start()
	log.Printf("[BillingSubConsumer] Started consuming topics: %v", topics)
	return consumer, nil
}
