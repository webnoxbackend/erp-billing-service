package services

import "example-service/internal/application/dto"

// ExampleService defines the interface for example business operations
type ExampleService interface {
	// CreateExample creates a new example
	CreateExample(req *dto.CreateExampleRequest) (*dto.ExampleResponse, error)

	// GetExample retrieves an example by ID
	GetExample(id int64) (*dto.ExampleResponse, error)

	// ListExamples retrieves all examples
	ListExamples() ([]*dto.ExampleResponse, error)

	// UpdateExample updates an existing example
	UpdateExample(id int64, req *dto.UpdateExampleRequest) (*dto.ExampleResponse, error)

	// DeleteExample deletes an example by ID
	DeleteExample(id int64) error
}

