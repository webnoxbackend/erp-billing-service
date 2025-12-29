package postgres

import (
	"example-service/internal/domain"
	"gorm.io/gorm"
)

// ExampleRepository implements the example repository interface using PostgreSQL
type ExampleRepository struct {
	db *gorm.DB
}

// NewExampleRepository creates a new PostgreSQL example repository
func NewExampleRepository(db *gorm.DB) *ExampleRepository {
	return &ExampleRepository{
		db: db,
	}
}

// Create creates a new example
func (r *ExampleRepository) Create(example *domain.Example) error {
	return r.db.Create(example).Error
}

// FindByID finds an example by ID
func (r *ExampleRepository) FindByID(id int64) (*domain.Example, error) {
	var example domain.Example
	if err := r.db.First(&example, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &example, nil
}

// FindAll finds all examples
func (r *ExampleRepository) FindAll() ([]*domain.Example, error) {
	var examples []*domain.Example
	if err := r.db.Find(&examples).Error; err != nil {
		return nil, err
	}
	return examples, nil
}

// Update updates an existing example
func (r *ExampleRepository) Update(example *domain.Example) error {
	return r.db.Save(example).Error
}

// Delete deletes an example by ID
func (r *ExampleRepository) Delete(id int64) error {
	return r.db.Delete(&domain.Example{}, id).Error
}

// Exists checks if an example exists with the given name
func (r *ExampleRepository) Exists(name string) (bool, error) {
	var count int64
	if err := r.db.Model(&domain.Example{}).Where("name = ?", name).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

