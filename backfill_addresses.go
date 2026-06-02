package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SourceAddress matches the schema of the source customer_addresses table in efcusdev
type SourceAddress struct {
	ID              string         `gorm:"type:uuid;primaryKey"`
	OrganizationID  string         `gorm:"type:uuid"`
	CustomerID      *string        `gorm:"type:uuid"`
	ContactID       *string        `gorm:"type:uuid"`
	Attention       *string        `gorm:"size:255"`
	Type            string         `gorm:"size:20"`
	Street1         string         `gorm:"size:255"`
	Street2         *string        `gorm:"size:255"`
	City            string         `gorm:"size:100"`
	State           string         `gorm:"size:100"`
	PostalCode      string         `gorm:"size:20"`
	Country         string         `gorm:"size:100"`
	Phone           *string        `gorm:"size:50"`
	Fax             *string        `gorm:"size:50"`
	IsDefault       bool           `json:"is_default"`
	Territory       *string        `gorm:"size:100"`
	Latitude        *float64       `gorm:"type:numeric(15,12)"`
	Longitude       *float64       `gorm:"type:numeric(15,12)"`
	NormalizedHash  *string        `gorm:"size:255"`
	GeocodingStatus *string        `gorm:"size:50"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index"`
}

func (SourceAddress) TableName() string {
	return "customer_addresses"
}

// TargetAddressReadOnly matches domain.AddressReadOnly in erp-billing-service
type TargetAddressReadOnly struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID  uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	CustomerID      *uuid.UUID `gorm:"type:uuid;index" json:"customer_id"`
	ContactID       *uuid.UUID `gorm:"type:uuid;index" json:"contact_id"`
	Attention       string     `gorm:"size:255" json:"attention"`
	Type            string     `gorm:"size:20;index" json:"type"`
	Street1         string     `gorm:"size:255" json:"street1"`
	Street2         string     `gorm:"size:255" json:"street2"`
	City            string     `gorm:"size:100" json:"city"`
	State           string     `gorm:"size:100" json:"state"`
	PostalCode      string     `gorm:"size:20" json:"postal_code"`
	Country         string     `gorm:"size:100" json:"country"`
	Phone           string     `gorm:"size:50" json:"phone"`
	Fax             string     `gorm:"size:50" json:"fax"`
	IsDefault       bool       `json:"is_default"`
	IsPrimary       bool       `json:"is_primary"`
	Territory       string     `gorm:"size:100" json:"territory"`
	Latitude        *float64   `gorm:"type:numeric(15,12)" json:"latitude"`
	Longitude       *float64   `gorm:"type:numeric(15,12)" json:"longitude"`
	NormalizedHash  string     `gorm:"size:255" json:"normalized_hash"`
	GeocodingStatus string     `gorm:"size:50" json:"geocoding_status"`
	Status          string     `gorm:"size:20;index" json:"status"`
	SyncedAt        time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetAddressReadOnly) TableName() string {
	return "addresses_readonly"
}

func parseUUID(s *string) *uuid.UUID {
	if s == nil || *s == "" {
		return nil
	}
	u, err := uuid.Parse(*s)
	if err != nil {
		return nil
	}
	return &u
}

func main() {
	sourceDSN := "postgresql://efcusdev:efcusdev@123@192.168.0.26:5456/efcusdev"
	targetDSN := "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"

	fmt.Println("Connecting to source database (efcusdev)...")
	srcDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to source DB: %v", err)
	}

	fmt.Println("Connecting to target database (efsbillingdevdb)...")
	tgtDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to target DB: %v", err)
	}

	fmt.Println("Fetching all customer addresses from source (including deleted)...")
	var srcAddrs []SourceAddress
	err = srcDB.Unscoped().Find(&srcAddrs).Error
	if err != nil {
		log.Fatalf("Failed to fetch source customer addresses: %v", err)
	}

	fmt.Printf("Found %d addresses in source. Syncing to target...\n", len(srcAddrs))

	successCount := 0
	for _, src := range srcAddrs {
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

		attention := ""
		if src.Attention != nil {
			attention = *src.Attention
		}
		street2 := ""
		if src.Street2 != nil {
			street2 = *src.Street2
		}
		phone := ""
		if src.Phone != nil {
			phone = *src.Phone
		}
		fax := ""
		if src.Fax != nil {
			fax = *src.Fax
		}
		territory := ""
		if src.Territory != nil {
			territory = *src.Territory
		}
		normalizedHash := ""
		if src.NormalizedHash != nil {
			normalizedHash = *src.NormalizedHash
		}
		geocodingStatus := ""
		if src.GeocodingStatus != nil {
			geocodingStatus = *src.GeocodingStatus
		}

		status := "active"
		if src.DeletedAt.Valid {
			status = "deleted"
		}

		tgt := TargetAddressReadOnly{
			ID:              srcID,
			OrganizationID:  orgID,
			CustomerID:      parseUUID(src.CustomerID),
			ContactID:       parseUUID(src.ContactID),
			Attention:       attention,
			Type:            src.Type,
			Street1:         src.Street1,
			Street2:         street2,
			City:            src.City,
			State:           src.State,
			PostalCode:      src.PostalCode,
			Country:         src.Country,
			Phone:           phone,
			Fax:             fax,
			IsDefault:       src.IsDefault,
			IsPrimary:       src.IsDefault,
			Territory:       territory,
			Latitude:        src.Latitude,
			Longitude:       src.Longitude,
			NormalizedHash:  normalizedHash,
			GeocodingStatus: geocodingStatus,
			Status:          status,
			SyncedAt:        time.Now().UTC(),
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync address %s: %v", src.ID, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Sync complete. Successfully synced %d / %d records.\n", successCount, len(srcAddrs))
}
