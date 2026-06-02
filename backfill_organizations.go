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

// SourceOrganization matches the schema of the source organizations table in efsorgdbdev
type SourceOrganization struct {
	ID               string    `gorm:"type:uuid;primaryKey"`
	ProfileID        string    `gorm:"type:varchar(50)"`
	OrganizationName string    `gorm:"type:varchar(255)"`
	OrganizationType string    `gorm:"type:varchar(100)"`
	Address          string    `gorm:"type:text"`
	City             string    `gorm:"type:varchar(100)"`
	State            string    `gorm:"type:varchar(100)"`
	ZipCode          string    `gorm:"type:varchar(20)"`
	Country          string    `gorm:"type:varchar(100)"`
	Phone            string    `gorm:"type:varchar(50)"`
	Website          string    `gorm:"type:text"`
	Currency         string    `gorm:"type:varchar(10)"`
	IsActive         bool      `gorm:"default:true"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	Timezone         string    `gorm:"type:varchar(100)"`
}

func (SourceOrganization) TableName() string {
	return "organizations"
}

// TargetOrganizationRM matches domain.OrganizationRM in erp-billing-service
type TargetOrganizationRM struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	ProfileID        string    `gorm:"type:varchar(50);not null" json:"profile_id"`
	OrganizationName string    `gorm:"type:varchar(255);not null" json:"organization_name"`
	OrganizationType string    `gorm:"type:varchar(100)" json:"organization_type"`
	Address          string    `gorm:"type:text" json:"address"`
	City             string    `gorm:"type:varchar(100)" json:"city"`
	State            string    `gorm:"type:varchar(100)" json:"state"`
	ZipCode          string    `gorm:"type:varchar(20)" json:"zip_code"`
	Country          string    `gorm:"type:varchar(100)" json:"country"`
	Phone            string    `gorm:"type:varchar(50)" json:"phone"`
	Website          string    `gorm:"type:text" json:"website"`
	Currency         string    `gorm:"type:varchar(10)" json:"currency"`
	Timezone         string    `gorm:"type:varchar(100)" json:"timezone"`
	IsActive         bool      `gorm:"default:true" json:"is_active"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	SyncedAt         time.Time `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetOrganizationRM) TableName() string {
	return "organization_readonly"
}

func main() {
	_ = godotenv.Load()
	sourceDSN := os.Getenv("SOURCE_DATABASE_URL")
	if sourceDSN == "" {
		sourceDSN = "postgresql://efsorgdbdev:efsorgdbdev@123@192.168.0.26:5455/efsorgdbdev"
	}
	targetDSN := os.Getenv("DATABASE_URL")
	if targetDSN == "" {
		targetDSN = "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"
	}

	fmt.Println("Connecting to source database (efsorgdbdev)...")
	srcDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to source DB: %v", err)
	}

	fmt.Println("Connecting to target database (efsbillingdevdb)...")
	tgtDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to target DB: %v", err)
	}

	fmt.Println("Fetching all organizations from source...")
	var srcOrgs []SourceOrganization
	err = srcDB.Find(&srcOrgs).Error
	if err != nil {
		log.Fatalf("Failed to fetch source organizations: %v", err)
	}

	fmt.Printf("Found %d organizations in source. Syncing to target...\n", len(srcOrgs))

	successCount := 0
	for _, src := range srcOrgs {
		srcID, err := uuid.Parse(src.ID)
		if err != nil {
			log.Printf("Invalid source ID: %s, error: %v", src.ID, err)
			continue
		}

		tgt := TargetOrganizationRM{
			ID:               srcID,
			ProfileID:        src.ProfileID,
			OrganizationName: src.OrganizationName,
			OrganizationType: src.OrganizationType,
			Address:          src.Address,
			City:             src.City,
			State:            src.State,
			ZipCode:          src.ZipCode,
			Country:          src.Country,
			Phone:            src.Phone,
			Website:          src.Website,
			Currency:         src.Currency,
			Timezone:         src.Timezone,
			IsActive:         src.IsActive,
			CreatedAt:        src.CreatedAt,
			UpdatedAt:        src.UpdatedAt,
			SyncedAt:         time.Now().UTC(),
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync organization %s: %v", src.ID, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Sync complete. Successfully synced %d / %d records.\n", successCount, len(srcOrgs))
}
