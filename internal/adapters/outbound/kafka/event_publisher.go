package kafka

import (
	"context"

	shared_events "github.com/efs/shared-events"
	shared_kafka "github.com/efs/shared-kafka"
)

type EventPublisher struct {
	producer shared_kafka.Producer
}

func NewEventPublisher(producer shared_kafka.Producer) *EventPublisher {
	return &EventPublisher{producer: producer}
}

func (p *EventPublisher) Publish(ctx context.Context, metadata shared_events.EventMetadata, payload interface{}) error {
	data, err := shared_events.Marshal(metadata, payload)
	if err != nil {
		return err
	}

	topic := shared_events.GetTopicForEventType(metadata.EventType)
	return p.producer.Publish(ctx, topic, metadata.AggregateID, data)
}
