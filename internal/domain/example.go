package domain

import "time"

// Example represents the core example entity in the domain
type Example struct {
	ID        int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Name      string    `gorm:"type:varchar(255);not null" json:"name"`
	Status    string    `gorm:"type:varchar(50);default:'active'" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// IsActive checks if the example is active
func (e *Example) IsActive() bool {
	return e.Status == "active"
}

// Activate marks the example as active
func (e *Example) Activate() {
	e.Status = "active"
	e.UpdatedAt = time.Now()
}

// Deactivate marks the example as inactive
func (e *Example) Deactivate() {
	e.Status = "inactive"
	e.UpdatedAt = time.Now()
}

// TableName specifies the table name for GORM
func (Example) TableName() string {
	return "examples"
}

