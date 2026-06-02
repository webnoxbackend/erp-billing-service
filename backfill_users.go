package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SourceUser matches the schema of the source users table in efsauthdevdb
type SourceUser struct {
	ID               string         `gorm:"type:uuid;primaryKey"`
	Email            string         `gorm:"type:varchar(255);not null"`
	PasswordHash     string         `gorm:"type:varchar(255);not null"`
	FirstName        string         `gorm:"type:varchar(100)"`
	LastName         string         `gorm:"type:varchar(100)"`
	FullName         string         `gorm:"type:varchar(255)"`
	Role             string         `gorm:"type:varchar(50);default:'user'"`
	OrganizationID   *string        `gorm:"type:varchar(255)"`
	OrganizationName string         `gorm:"type:varchar(255)"`
	ProfileID        *string        `gorm:"type:varchar(50)"`
	EmployeeID       *string        `gorm:"type:varchar(50)"`
	WorkforceUserID  *string        `gorm:"type:uuid"`
	CustomerID       *string        `gorm:"type:varchar(50)"`
	UserType         string         `gorm:"type:varchar(20);default:'regular'"`
	ProfilePhotoURL  string         `gorm:"type:varchar(255)"`
	PhoneNumber      *string        `gorm:"type:varchar(20)"`
	PinHash          string         `gorm:"type:varchar(255)"`
	EmailVerified    *bool          `gorm:"default:false"`
	IsActive         *bool          `gorm:"default:false"`
	InvitationToken  *string        `gorm:"type:varchar(255)"`
	InvitationExpiry *time.Time     `gorm:"type:timestamp"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index"`
}

func (SourceUser) TableName() string {
	return "users"
}

// TargetUserReadOnly matches users_readonly in efsbillingdevdb
type TargetUserReadOnly struct {
	ID               string     `gorm:"type:uuid;primaryKey" json:"id"`
	Email            string     `gorm:"type:varchar(255);not null" json:"email"`
	PasswordHash     string     `gorm:"type:varchar(255);not null" json:"-"`
	FirstName        string     `gorm:"type:varchar(100)" json:"first_name"`
	LastName         string     `gorm:"type:varchar(100)" json:"last_name"`
	FullName         string     `gorm:"type:varchar(255)" json:"full_name"`
	Role             string     `gorm:"type:varchar(50);default:'user'" json:"role"`
	OrganizationID   *string    `gorm:"type:varchar(255)" json:"organization_id,omitempty"`
	OrganizationName string     `gorm:"type:varchar(255)" json:"organization_name,omitempty"`
	ProfileID        *string    `gorm:"type:varchar(50)" json:"profile_id,omitempty"`
	EmployeeID       *string    `gorm:"type:varchar(50)" json:"employee_id,omitempty"`
	WorkforceUserID  *string    `gorm:"type:uuid" json:"workforce_user_id,omitempty"`
	CustomerID       *string    `gorm:"type:varchar(50)" json:"customer_id,omitempty"`
	UserType         string     `gorm:"type:varchar(20);default:'regular'" json:"user_type"`
	ProfilePhotoURL  string     `gorm:"type:varchar(255)" json:"profile_photo_url"`
	PhoneNumber      *string    `gorm:"type:varchar(20)" json:"phone_number,omitempty"`
	PinHash          string     `gorm:"type:varchar(255)" json:"-"`
	EmailVerified    *bool      `gorm:"default:false" json:"email_verified"`
	IsActive         *bool      `gorm:"default:false" json:"is_active"`
	InvitationToken  *string    `gorm:"type:varchar(255)" json:"invitation_token,omitempty"`
	InvitationExpiry *time.Time `json:"invitation_expiry,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt         time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetUserReadOnly) TableName() string {
	return "users_readonly"
}

func main() {
	_ = godotenv.Load()
	sourceDSN := os.Getenv("SOURCE_DATABASE_URL")
	if sourceDSN == "" {
		sourceDSN = "postgresql://efsauthdevdb:efsauthdevdb@123@192.168.0.26:5466/efsauthdevdb"
	}
	targetDSN := os.Getenv("DATABASE_URL")
	if targetDSN == "" {
		targetDSN = "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"
	}

	fmt.Println("Connecting to source auth database (efsauthdevdb)...")
	srcDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to source DB: %v", err)
	}

	fmt.Println("Connecting to target billing database (efsbillingdevdb)...")
	tgtDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to target DB: %v", err)
	}

	// Make sure the target table exists
	fmt.Println("Auto-migrating target users_readonly table...")
	err = tgtDB.AutoMigrate(&TargetUserReadOnly{})
	if err != nil {
		log.Fatalf("Failed to auto-migrate users_readonly: %v", err)
	}

	fmt.Println("Fetching all users from source auth database (including deleted)...")
	var srcUsers []SourceUser
	err = srcDB.Unscoped().Find(&srcUsers).Error
	if err != nil {
		log.Fatalf("Failed to fetch source users: %v", err)
	}

	fmt.Printf("Found %d users in source database. Syncing to target table users_readonly...\n", len(srcUsers))

	successCount := 0
	for _, src := range srcUsers {
		var deletedAt *time.Time
		if src.DeletedAt.Valid {
			deletedAt = &src.DeletedAt.Time
		}

		tgt := TargetUserReadOnly{
			ID:               src.ID,
			Email:            src.Email,
			PasswordHash:     src.PasswordHash,
			FirstName:        src.FirstName,
			LastName:         src.LastName,
			FullName:         src.FullName,
			Role:             src.Role,
			OrganizationID:   src.OrganizationID,
			OrganizationName: src.OrganizationName,
			ProfileID:        src.ProfileID,
			EmployeeID:       src.EmployeeID,
			WorkforceUserID:  src.WorkforceUserID,
			CustomerID:       src.CustomerID,
			UserType:         src.UserType,
			ProfilePhotoURL:  src.ProfilePhotoURL,
			PhoneNumber:      src.PhoneNumber,
			PinHash:          src.PinHash,
			EmailVerified:    src.EmailVerified,
			IsActive:         src.IsActive,
			InvitationToken:  src.InvitationToken,
			InvitationExpiry: src.InvitationExpiry,
			CreatedAt:        src.CreatedAt,
			UpdatedAt:        src.UpdatedAt,
			DeletedAt:        deletedAt,
			SyncedAt:         time.Now().UTC(),
		}

		// Use Save (upsert)
		err = tgtDB.Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync user %s: %v", src.Email, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Sync complete. Successfully migrated %d / %d records to users_readonly.\n", successCount, len(srcUsers))
}
