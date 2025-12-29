package application

import (
	"example-service/internal/application/dto"
	"example-service/internal/domain"
	"example-service/internal/ports/external"
	"example-service/internal/ports/repositories"
	"example-service/internal/ports/services"
	"fmt"
	"time"
)

// ExampleService implements the example service interface
type ExampleService struct {
	exampleRepo   repositories.ExampleRepository
	eventPublisher external.EventPublisher
}

// NewExampleService creates a new example service
func NewExampleService(
	exampleRepo repositories.ExampleRepository,
	eventPublisher external.EventPublisher,
) services.ExampleService {
	return &ExampleService{
		exampleRepo:   exampleRepo,
		eventPublisher: eventPublisher,
	}
}

// CreateExample creates a new example
func (s *ExampleService) CreateExample(req *dto.CreateExampleRequest) (*dto.ExampleResponse, error) {
	// Check if example already exists
	exists, err := s.exampleRepo.Exists(req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check if example exists: %w", err)
	}
	if exists {
		return nil, domain.ErrExampleAlreadyExists
	}

	// Create domain entity
	now := time.Now()
	example := &domain.Example{
		Name:      req.Name,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Save to repository
	if err := s.exampleRepo.Create(example); err != nil {
		return nil, fmt.Errorf("failed to create example: %w", err)
	}

	// Publish event
	if s.eventPublisher != nil {
		event := &domain.Event{
			Type:      "ExampleCreated",
			Payload:   domain.ExampleCreatedEvent{ExampleID: example.ID, Name: example.Name, Timestamp: now},
			Timestamp: now,
		}
		_ = s.eventPublisher.Publish(event) // Log error but don't fail
	}

	// Return response
	return s.toDTO(example), nil
}

// GetExample retrieves an example by ID
func (s *ExampleService) GetExample(id int64) (*dto.ExampleResponse, error) {
	example, err := s.exampleRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get example: %w", err)
	}
	if example == nil {
		return nil, domain.ErrExampleNotFound
	}

	return s.toDTO(example), nil
}

// ListExamples retrieves all examples
func (s *ExampleService) ListExamples() ([]*dto.ExampleResponse, error) {
	examples, err := s.exampleRepo.FindAll()
	if err != nil {
		return nil, fmt.Errorf("failed to list examples: %w", err)
	}

	dtos := make([]*dto.ExampleResponse, len(examples))
	for i, example := range examples {
		dtos[i] = s.toDTO(example)
	}

	return dtos, nil
}

// UpdateExample updates an existing example
func (s *ExampleService) UpdateExample(id int64, req *dto.UpdateExampleRequest) (*dto.ExampleResponse, error) {
	// Get existing example
	example, err := s.exampleRepo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get example: %w", err)
	}
	if example == nil {
		return nil, domain.ErrExampleNotFound
	}

	// Update fields
	if req.Name != "" {
		example.Name = req.Name
	}
	if req.Status != "" {
		example.Status = req.Status
	}
	example.UpdatedAt = time.Now()

	// Save to repository
	if err := s.exampleRepo.Update(example); err != nil {
		return nil, fmt.Errorf("failed to update example: %w", err)
	}

	// Publish event
	if s.eventPublisher != nil {
		event := &domain.Event{
			Type:      "ExampleUpdated",
			Payload:   domain.ExampleUpdatedEvent{ExampleID: example.ID, Name: example.Name, Timestamp: time.Now()},
			Timestamp: time.Now(),
		}
		_ = s.eventPublisher.Publish(event)
	}

	return s.toDTO(example), nil
}

// DeleteExample deletes an example by ID
func (s *ExampleService) DeleteExample(id int64) error {
	// Check if example exists
	example, err := s.exampleRepo.FindByID(id)
	if err != nil {
		return fmt.Errorf("failed to get example: %w", err)
	}
	if example == nil {
		return domain.ErrExampleNotFound
	}

	// Delete from repository
	if err := s.exampleRepo.Delete(id); err != nil {
		return fmt.Errorf("failed to delete example: %w", err)
	}

	// Publish event
	if s.eventPublisher != nil {
		event := &domain.Event{
			Type:      "ExampleDeleted",
			Payload:   domain.ExampleDeletedEvent{ExampleID: id, Timestamp: time.Now()},
			Timestamp: time.Now(),
		}
		_ = s.eventPublisher.Publish(event)
	}

	return nil
}

// toDTO converts a domain entity to a DTO
func (s *ExampleService) toDTO(example *domain.Example) *dto.ExampleResponse {
	return &dto.ExampleResponse{
		ID:        example.ID,
		Name:      example.Name,
		Status:    example.Status,
		CreatedAt: example.CreatedAt.Format(time.RFC3339),
		UpdatedAt: example.UpdatedAt.Format(time.RFC3339),
	}
}

