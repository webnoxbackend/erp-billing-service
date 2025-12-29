package domain

import "time"

// Event represents a domain event
type Event struct {
	Type      string
	Payload   interface{}
	Timestamp time.Time
}

// ExampleCreatedEvent represents an example creation event
type ExampleCreatedEvent struct {
	ExampleID int64
	Name      string
	Timestamp time.Time
}

// ExampleUpdatedEvent represents an example update event
type ExampleUpdatedEvent struct {
	ExampleID int64
	Name      string
	Timestamp time.Time
}

// ExampleDeletedEvent represents an example deletion event
type ExampleDeletedEvent struct {
	ExampleID int64
	Timestamp time.Time
}

