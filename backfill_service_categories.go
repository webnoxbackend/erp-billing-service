package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SourceServiceCategory matches the schema of the source service_categories table in efsspdbdev
type SourceServiceCategory struct {
	ID             string         `gorm:"type:uuid;primaryKey"`
	OrganizationID string         `gorm:"type:uuid"`
	CategoryName   string         `gorm:"size:255"`
	CategoryCode   string         `gorm:"size:100"`
	Description    string         `gorm:"type:text"`
	Type           string         `gorm:"size:20"`
	ImagePath      string         `gorm:"type:text"`
	Status         string         `gorm:"size:20"`
	CreatedBy      *string        `gorm:"type:uuid"`
	UpdatedBy      *string        `gorm:"type:uuid"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`
}

func (SourceServiceCategory) TableName() string {
	return "service_categories"
}

// TargetServiceCategoryReadOnly matches domain.ServiceCategoryReadOnly in erp-billing-service
type TargetServiceCategoryReadOnly struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID uuid.UUID  `gorm:"type:uuid;not null;index" json:"organization_id"`
	CategoryName   string     `gorm:"type:varchar(255);not null" json:"category_name"`
	CategoryCode   string     `gorm:"type:varchar(100);not null" json:"category_code"`
	Description    string     `gorm:"type:text" json:"description"`
	Type           string     `gorm:"type:varchar(20);not null;default:'service'" json:"type"`
	ImagePath      string     `gorm:"type:text" json:"image_path"`
	Status         string     `gorm:"type:varchar(20);default:'ACTIVE'" json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	SyncedAt       time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
	DeletedAt      *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (TargetServiceCategoryReadOnly) TableName() string {
	return "service_categories_readonly"
}

func main() {
	_ = godotenv.Load()
	sourceDSN := os.Getenv("SOURCE_DATABASE_URL")
	if sourceDSN == "" {
		sourceDSN = "postgresql://efsspdbdev:efsspdbdev@123@192.168.0.26:5458/efsspdbdev"
	}
	targetDSN := os.Getenv("DATABASE_URL")
	if targetDSN == "" {
		targetDSN = "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"
	}

	fmt.Println("Connecting to source database (efsspdbdev)...")
	srcDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to source DB: %v", err)
	}

	fmt.Println("Connecting to target database (efsbillingdevdb)...")
	tgtDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to target DB: %v", err)
	}

	fmt.Println("Fetching all service categories from source (including deleted)...")
	var srcCategories []SourceServiceCategory
	err = srcDB.Unscoped().Find(&srcCategories).Error
	if err != nil {
		log.Fatalf("Failed to fetch source service categories: %v", err)
	}

	fmt.Printf("Found %d service categories in source. Syncing to target...\n", len(srcCategories))

	successCount := 0
	for _, src := range srcCategories {
		srcID, err := uuid.Parse(src.ID)
		if err != nil {
			log.Printf("Invalid source ID: %s, error: %v", src.ID, err)
			continue
		}

		orgID, err := uuid.Parse(src.OrganizationID)
		if err != nil {
			log.Printf("Invalid organization ID: %s, error: %v", src.OrganizationID, err)
			continue
		}

		var deletedAt *time.Time
		if src.DeletedAt.Valid {
			deletedAt = &src.DeletedAt.Time
		}

		tgt := TargetServiceCategoryReadOnly{
			ID:             srcID,
			OrganizationID: orgID,
			CategoryName:   src.CategoryName,
			CategoryCode:   src.CategoryCode,
			Description:    src.Description,
			Type:           src.Type,
			ImagePath:      src.ImagePath,
			Status:         src.Status,
			CreatedAt:      src.CreatedAt,
			UpdatedAt:      src.UpdatedAt,
			SyncedAt:       time.Now().UTC(),
			DeletedAt:      deletedAt,
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync service category %s: %v", src.ID, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Sync complete. Successfully synced %d / %d records.\n", successCount, len(srcCategories))
}
