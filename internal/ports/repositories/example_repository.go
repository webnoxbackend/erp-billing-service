package repositories

import "example-service/internal/domain"

// ExampleRepository defines the interface for example data operations
type ExampleRepository interface {
	// Create creates a new example
	Create(example *domain.Example) error

	// FindByID finds an example by ID
	FindByID(id int64) (*domain.Example, error)

	// FindAll finds all examples
	FindAll() ([]*domain.Example, error)

	// Update updates an existing example
	Update(example *domain.Example) error

	// Delete deletes an example by ID
	Delete(id int64) error

	// Exists checks if an example exists with the given name
	Exists(name string) (bool, error)
}

