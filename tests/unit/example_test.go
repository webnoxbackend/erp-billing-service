package unit

import (
	"example-service/internal/domain"
	"testing"
)

// TestExample_IsActive tests the IsActive method
func TestExample_IsActive(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{
			name:   "active status returns true",
			status: "active",
			want:   true,
		},
		{
			name:   "inactive status returns false",
			status: "inactive",
			want:   false,
		},
		{
			name:   "empty status returns false",
			status: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			example := &domain.Example{
				Status: tt.status,
			}
			if got := example.IsActive(); got != tt.want {
				t.Errorf("Example.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExample_Activate tests the Activate method
func TestExample_Activate(t *testing.T) {
	example := &domain.Example{
		Status: "inactive",
	}
	example.Activate()
	if example.Status != "active" {
		t.Errorf("Expected status to be 'active', got %s", example.Status)
	}
}

// TestExample_Deactivate tests the Deactivate method
func TestExample_Deactivate(t *testing.T) {
	example := &domain.Example{
		Status: "active",
	}
	example.Deactivate()
	if example.Status != "inactive" {
		t.Errorf("Expected status to be 'inactive', got %s", example.Status)
	}
}

