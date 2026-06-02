package main

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, j)
}

// SourceItem matches the unified item entity schema in efsspdbdev
type SourceItem struct {
	ID               string     `gorm:"type:uuid;primaryKey"`
	OrganizationID   string     `gorm:"type:uuid"`
	SKU              string     `gorm:"type:varchar(255)"`
	Name             string     `gorm:"type:varchar(255)"`
	Description      string     `gorm:"type:text"`
	Type             string     `gorm:"type:varchar(20)"`
	Status           string     `gorm:"type:varchar(20)"`
	UnitID           *string    `gorm:"type:varchar(100)"`
	Dimensions       JSONB      `gorm:"type:jsonb"`
	Weight           JSONB      `gorm:"type:jsonb"`
	ManufacturerID   *string    `gorm:"type:varchar(255)"`
	BrandID          *string    `gorm:"type:uuid"`
	BarCode          string     `gorm:"type:varchar(255)"`
	UPC              string     `gorm:"type:varchar(100)"`
	EAN              string     `gorm:"type:varchar(100)"`
	ISBN             string     `gorm:"type:varchar(100)"`
	MPN              string     `gorm:"type:varchar(100)"`
	SalesInfo        JSONB      `gorm:"type:jsonb"`
	PurchaseInfo     JSONB      `gorm:"type:jsonb"`
	InventoryInfo    JSONB      `gorm:"type:jsonb"`
	ServiceInfo      JSONB      `gorm:"type:jsonb"`
	CRMProductInfo   JSONB      `gorm:"type:jsonb"`
	CRMFields        JSONB      `gorm:"type:jsonb"`
	CRMServiceFields JSONB      `gorm:"type:jsonb"`
	CreatedBy        *string    `gorm:"type:uuid"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedBy        *string    `gorm:"type:uuid"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	CategoryID       *string    `gorm:"type:uuid"`
}

func (SourceItem) TableName() string {
	return "items"
}

// TargetItemRM matches domain.ItemRM in erp-billing-service
type TargetItemRM struct {
	ID               uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID   uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	SKU              string     `gorm:"type:varchar(255)" json:"sku"`
	Name             string     `gorm:"type:varchar(255)" json:"name"`
	Description      string     `gorm:"type:text" json:"description"`
	Type             string     `gorm:"type:varchar(20)" json:"type"`
	Status           string     `gorm:"type:varchar(20)" json:"status"`
	UnitID           *string    `gorm:"type:varchar(100)" json:"unit_id"`
	Dimensions       JSONB      `gorm:"type:jsonb" json:"dimensions"`
	Weight           JSONB      `gorm:"type:jsonb" json:"weight"`
	ManufacturerID   *string    `gorm:"type:varchar(255)" json:"manufacturer_id"`
	BrandID          *string    `gorm:"type:varchar(255)" json:"brand_id"`
	BarCode          string     `gorm:"type:varchar(255)" json:"bar_code"`
	UPC              string     `gorm:"type:varchar(100)" json:"upc"`
	EAN              string     `gorm:"type:varchar(100)" json:"ean"`
	ISBN             string     `gorm:"type:varchar(100)" json:"isbn"`
	MPN              string     `gorm:"type:varchar(100)" json:"mpn"`
	SalesInfo        JSONB      `gorm:"type:jsonb" json:"sales_info"`
	PurchaseInfo     JSONB      `gorm:"type:jsonb" json:"purchase_info"`
	InventoryInfo    JSONB      `gorm:"type:jsonb" json:"inventory_info"`
	ServiceInfo      JSONB      `gorm:"type:jsonb" json:"service_info"`
	CRMProductInfo   JSONB      `gorm:"type:jsonb" json:"crm_product_info"`
	CRMFields        JSONB      `gorm:"type:jsonb" json:"crm_fields"`
	CRMServiceFields JSONB      `gorm:"type:jsonb" json:"crm_service_fields"`
	CreatedBy        *string    `gorm:"type:varchar(255)" json:"created_by"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedBy        *string    `gorm:"type:varchar(255)" json:"updated_by"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
	CategoryID       *string    `gorm:"type:varchar(255)" json:"category_id"`
	SyncedAt         time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetItemRM) TableName() string {
	return "items_readonly"
}

func main() {
	sourceDSN := "postgresql://efsspdbdev:efsspdbdev@123@192.168.0.26:5458/efsspdbdev"
	targetDSN := "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"

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

	fmt.Println("Fetching all items from source (including deleted)...")
	var srcItems []SourceItem
	err = srcDB.Unscoped().Find(&srcItems).Error
	if err != nil {
		log.Fatalf("Failed to fetch source items: %v", err)
	}

	fmt.Printf("Found %d items in source. Syncing to target...\n", len(srcItems))

	successCount := 0
	for _, src := range srcItems {
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

		tgt := TargetItemRM{
			ID:               srcID,
			OrganizationID:   orgID,
			SKU:              src.SKU,
			Name:             src.Name,
			Description:      src.Description,
			Type:             src.Type,
			Status:           src.Status,
			UnitID:           src.UnitID,
			Dimensions:       src.Dimensions,
			Weight:           src.Weight,
			ManufacturerID:   src.ManufacturerID,
			BrandID:          src.BrandID,
			BarCode:          src.BarCode,
			UPC:              src.UPC,
			EAN:              src.EAN,
			ISBN:             src.ISBN,
			MPN:              src.MPN,
			SalesInfo:        src.SalesInfo,
			PurchaseInfo:     src.PurchaseInfo,
			InventoryInfo:    src.InventoryInfo,
			ServiceInfo:      src.ServiceInfo,
			CRMProductInfo:   src.CRMProductInfo,
			CRMFields:        src.CRMFields,
			CRMServiceFields: src.CRMServiceFields,
			CreatedBy:        src.CreatedBy,
			CreatedAt:        src.CreatedAt,
			UpdatedBy:        src.UpdatedBy,
			UpdatedAt:        src.UpdatedAt,
			DeletedAt:        src.DeletedAt,
			CategoryID:       src.CategoryID,
			SyncedAt:         time.Now().UTC(),
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync item %s: %v", src.ID, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Sync complete. Successfully synced %d / %d records.\n", successCount, len(srcItems))
}
