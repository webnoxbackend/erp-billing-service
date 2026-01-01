package domain

import (
	"context"

	shared_events "github.com/efs/shared-events"
)

type EventPublisher interface {
	Publish(ctx context.Context, metadata shared_events.EventMetadata, payload interface{}) error
}
