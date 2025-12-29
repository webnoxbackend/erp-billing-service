package domain

import "errors"

// Domain errors
var (
	ErrExampleNotFound      = errors.New("example not found")
	ErrExampleAlreadyExists = errors.New("example already exists")
	ErrInvalidInput         = errors.New("invalid input")
)

