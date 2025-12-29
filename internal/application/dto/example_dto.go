package dto

// CreateExampleRequest represents the request to create an example
type CreateExampleRequest struct {
	Name string `json:"name" validate:"required,min=1,max=255"`
}

// UpdateExampleRequest represents the request to update an example
type UpdateExampleRequest struct {
	Name   string `json:"name" validate:"omitempty,min=1,max=255"`
	Status string `json:"status" validate:"omitempty,oneof=active inactive"`
}

// ExampleResponse represents the example response
type ExampleResponse struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

