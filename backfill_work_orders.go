package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SourceWorkOrder matches the schema of the source work_orders table in efswomdbdev
type SourceWorkOrder struct {
	ID                string         `gorm:"type:uuid;primaryKey"`
	OrganizationID    string         `gorm:"type:varchar(50)"`
	RequestID         *string        `gorm:"type:uuid"`
	EstimateID        *string        `gorm:"type:uuid"`
	ServiceCategoryID *string        `gorm:"type:uuid"`
	Summary           string         `gorm:"size:255"`
	Priority          string         `gorm:"size:50"`
	Type              string         `gorm:"size:50"`
	DueDate           *time.Time     `gorm:"type:timestamp"`
	Status            string         `gorm:"size:50"`
	BillingStatus     string         `gorm:"size:50"`
	AssetID           *string        `gorm:"type:uuid"`
	RequiredSkillID   *string        `gorm:"type:uuid"`
	CustomerID        *string        `gorm:"type:uuid"`
	ContactID         *string        `gorm:"type:uuid"`
	Email             string         `gorm:"size:255"`
	Phone             string         `gorm:"size:50"`
	Mobile            string         `gorm:"size:50"`
	ServiceAddressID  *string        `gorm:"type:uuid"`
	BillingAddressID  *string        `gorm:"type:uuid"`
	PreferredDate1    *time.Time     `gorm:"type:timestamp"`
	PreferredDate2    *time.Time     `gorm:"type:timestamp"`
	PreferredTime     string         `gorm:"size:50"`
	PreferenceNote    string         `gorm:"type:text"`
	SubTotal          float64        `gorm:"type:decimal(15,2)"`
	Discount          float64        `gorm:"type:decimal(15,2)"`
	Adjustment        float64        `gorm:"type:decimal(15,2)"`
	GrandTotal        float64        `gorm:"type:decimal(15,2)"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

func (SourceWorkOrder) TableName() string {
	return "work_orders"
}

// SourceWorkOrderServiceLine matches the schema of the source work_order_service_lines table in efswomdbdev
type SourceWorkOrderServiceLine struct {
	ID          string         `gorm:"type:uuid;primaryKey"`
	WorkOrderID string         `gorm:"type:uuid"`
	ServiceID   *string        `gorm:"type:uuid"`
	Description string         `gorm:"type:text"`
	Quantity    float64        `gorm:"type:decimal(10,2)"`
	Unit        string         `gorm:"size:20"`
	ListPrice   float64        `gorm:"type:decimal(15,2)"`
	LineAmount  float64        `gorm:"type:decimal(15,2)"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (SourceWorkOrderServiceLine) TableName() string {
	return "work_order_service_lines"
}

// SourceWorkOrderPartLine matches the schema of the source work_order_part_lines table in efswomdbdev
type SourceWorkOrderPartLine struct {
	ID          string         `gorm:"type:uuid;primaryKey"`
	WorkOrderID string         `gorm:"type:uuid"`
	PartID      *string        `gorm:"type:uuid"`
	Description string         `gorm:"type:text"`
	Quantity    float64        `gorm:"type:decimal(10,2)"`
	Unit        string         `gorm:"size:20"`
	ListPrice   float64        `gorm:"type:decimal(15,2)"`
	LineAmount  float64        `gorm:"type:decimal(15,2)"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index"`
}

func (SourceWorkOrderPartLine) TableName() string {
	return "work_order_part_lines"
}

// TargetWorkOrderRM matches target work_orders_readonly
type TargetWorkOrderRM struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID    uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	RequestID         *uuid.UUID `gorm:"type:uuid;index" json:"request_id,omitempty"`
	EstimateID        *uuid.UUID `gorm:"type:uuid;index" json:"estimate_id,omitempty"`
	ServiceCategoryID *uuid.UUID `gorm:"type:uuid;index" json:"service_category_id,omitempty"`

	Summary           string     `gorm:"size:255;not null" json:"summary"`
	Priority          string     `gorm:"size:50" json:"priority,omitempty"`
	Type              string     `gorm:"size:50" json:"type,omitempty"`
	DueDate           *time.Time `json:"due_date,omitempty"`
	Status            string     `gorm:"size:50;default:'OPEN'" json:"status"`
	BillingStatus     string     `gorm:"size:50;default:'New'" json:"billing_status"`

	AssetID           *uuid.UUID `gorm:"type:uuid;index" json:"asset_id,omitempty"`
	RequiredSkillID   *uuid.UUID `gorm:"type:uuid;index" json:"required_skill_id,omitempty"`

	CustomerID        *uuid.UUID `gorm:"type:uuid" json:"customer_id,omitempty"`
	ContactID         *uuid.UUID `gorm:"type:uuid" json:"contact_id,omitempty"`
	Email             string     `gorm:"size:255" json:"email,omitempty"`
	Phone             string     `gorm:"size:50" json:"phone,omitempty"`
	Mobile            string     `gorm:"size:50" json:"mobile,omitempty"`

	ServiceAddressID  *uuid.UUID `gorm:"type:uuid;index" json:"service_address_id,omitempty"`
	BillingAddressID  *uuid.UUID `gorm:"type:uuid;index" json:"billing_address_id,omitempty"`

	PreferredDate1    *time.Time `gorm:"type:timestamp" json:"preferred_date1,omitempty"`
	PreferredDate2    *time.Time `gorm:"type:timestamp" json:"preferred_date2,omitempty"`
	PreferredTime     string     `gorm:"size:50" json:"preferred_time,omitempty"`
	PreferenceNote    string     `gorm:"type:text" json:"preference_note,omitempty"`

	SubTotal          float64    `gorm:"type:decimal(15,2);default:0" json:"sub_total"`
	Discount          float64    `gorm:"type:decimal(15,2);default:0" json:"discount"`
	Adjustment        float64    `gorm:"type:decimal(15,2);default:0" json:"adjustment"`
	GrandTotal        float64    `gorm:"type:decimal(15,2);default:0" json:"grand_total"`

	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt          time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetWorkOrderRM) TableName() string {
	return "work_orders_readonly"
}

// TargetWorkOrderServiceLineRM matches target work_order_service_lines_readonly
type TargetWorkOrderServiceLineRM struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkOrderID uuid.UUID  `gorm:"type:uuid;index" json:"work_order_id"`
	ServiceID   *uuid.UUID `gorm:"type:uuid" json:"service_id,omitempty"`
	Description string     `gorm:"type:text" json:"description"`
	Quantity    float64    `gorm:"type:decimal(10,2)" json:"quantity"`
	Unit        string     `gorm:"size:20" json:"unit,omitempty"`
	ListPrice   float64    `gorm:"type:decimal(15,2)" json:"list_price"`
	LineAmount  float64    `gorm:"type:decimal(15,2)" json:"line_amount"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt    time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetWorkOrderServiceLineRM) TableName() string {
	return "work_order_service_lines_readonly"
}

// TargetWorkOrderPartLineRM matches target work_order_part_lines_readonly
type TargetWorkOrderPartLineRM struct {
	ID          uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	WorkOrderID uuid.UUID  `gorm:"type:uuid;index" json:"work_order_id"`
	PartID      *uuid.UUID `gorm:"type:uuid" json:"part_id,omitempty"`
	Description string     `gorm:"type:text" json:"description"`
	Quantity    float64    `gorm:"type:decimal(10,2)" json:"quantity"`
	Unit        string     `gorm:"size:20" json:"unit,omitempty"`
	ListPrice   float64    `gorm:"type:decimal(15,2)" json:"list_price"`
	LineAmount  float64    `gorm:"type:decimal(15,2)" json:"line_amount"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt    time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetWorkOrderPartLineRM) TableName() string {
	return "work_order_part_lines_readonly"
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
	sourceDSN := "postgresql://efswomdbdev:efswomdbdev@123@192.168.0.26:5460/efswomdbdev"
	targetDSN := "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"

	fmt.Println("Connecting to source database (efswomdbdev)...")
	srcDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to source DB: %v", err)
	}

	fmt.Println("Connecting to target database (efsbillingdevdb)...")
	tgtDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to target DB: %v", err)
	}

	fmt.Println("Auto-migrating target tables...")
	err = tgtDB.AutoMigrate(&TargetWorkOrderRM{}, &TargetWorkOrderServiceLineRM{}, &TargetWorkOrderPartLineRM{})
	if err != nil {
		log.Fatalf("Failed to auto-migrate target tables: %v", err)
	}

	// 1. Migrate Work Orders
	fmt.Println("Fetching all work orders from source...")
	var srcWorkOrders []SourceWorkOrder
	err = srcDB.Unscoped().Find(&srcWorkOrders).Error
	if err != nil {
		log.Fatalf("Failed to fetch source work orders: %v", err)
	}
	fmt.Printf("Found %d work orders in source. Syncing to target...\n", len(srcWorkOrders))

	now := time.Now().UTC()
	woSuccess := 0
	for _, src := range srcWorkOrders {
		id, err := uuid.Parse(src.ID)
		if err != nil {
			log.Printf("Invalid source WO ID: %s, error: %v", src.ID, err)
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

		tgt := TargetWorkOrderRM{
			ID:                id,
			OrganizationID:    orgID,
			RequestID:         parseUUID(src.RequestID),
			EstimateID:        parseUUID(src.EstimateID),
			ServiceCategoryID: parseUUID(src.ServiceCategoryID),
			Summary:           src.Summary,
			Priority:          src.Priority,
			Type:              src.Type,
			DueDate:           src.DueDate,
			Status:            src.Status,
			BillingStatus:     src.BillingStatus,
			AssetID:           parseUUID(src.AssetID),
			RequiredSkillID:   parseUUID(src.RequiredSkillID),
			CustomerID:        parseUUID(src.CustomerID),
			ContactID:         parseUUID(src.ContactID),
			Email:             src.Email,
			Phone:             src.Phone,
			Mobile:            src.Mobile,
			ServiceAddressID:  parseUUID(src.ServiceAddressID),
			BillingAddressID:  parseUUID(src.BillingAddressID),
			PreferredDate1:    src.PreferredDate1,
			PreferredDate2:    src.PreferredDate2,
			PreferredTime:     src.PreferredTime,
			PreferenceNote:    src.PreferenceNote,
			SubTotal:          src.SubTotal,
			Discount:          src.Discount,
			Adjustment:        src.Adjustment,
			GrandTotal:        src.GrandTotal,
			CreatedAt:         src.CreatedAt,
			UpdatedAt:         src.UpdatedAt,
			DeletedAt:         deletedAt,
			SyncedAt:          now,
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync work order %s: %v", src.ID, err)
		} else {
			woSuccess++
		}
	}
	fmt.Printf("Work orders sync complete. Success: %d / %d\n", woSuccess, len(srcWorkOrders))

	// 2. Migrate Service Lines
	fmt.Println("Fetching all service lines from source...")
	var srcServiceLines []SourceWorkOrderServiceLine
	err = srcDB.Unscoped().Find(&srcServiceLines).Error
	if err != nil {
		log.Fatalf("Failed to fetch source service lines: %v", err)
	}
	fmt.Printf("Found %d service lines in source. Syncing to target...\n", len(srcServiceLines))

	slSuccess := 0
	for _, src := range srcServiceLines {
		id, err := uuid.Parse(src.ID)
		if err != nil {
			log.Printf("Invalid service line ID: %s, error: %v", src.ID, err)
			continue
		}
		woID, err := uuid.Parse(src.WorkOrderID)
		if err != nil {
			log.Printf("Invalid work order ID for service line %s: %s, error: %v", src.ID, src.WorkOrderID, err)
			continue
		}

		var deletedAt *time.Time
		if src.DeletedAt.Valid {
			deletedAt = &src.DeletedAt.Time
		}

		tgt := TargetWorkOrderServiceLineRM{
			ID:          id,
			WorkOrderID: woID,
			ServiceID:   parseUUID(src.ServiceID),
			Description: src.Description,
			Quantity:    src.Quantity,
			Unit:        src.Unit,
			ListPrice:   src.ListPrice,
			LineAmount:  src.LineAmount,
			CreatedAt:   src.CreatedAt,
			UpdatedAt:   src.UpdatedAt,
			DeletedAt:   deletedAt,
			SyncedAt:    now,
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync service line %s: %v", src.ID, err)
		} else {
			slSuccess++
		}
	}
	fmt.Printf("Service lines sync complete. Success: %d / %d\n", slSuccess, len(srcServiceLines))

	// 3. Migrate Part Lines
	fmt.Println("Fetching all part lines from source...")
	var srcPartLines []SourceWorkOrderPartLine
	err = srcDB.Unscoped().Find(&srcPartLines).Error
	if err != nil {
		log.Fatalf("Failed to fetch source part lines: %v", err)
	}
	fmt.Printf("Found %d part lines in source. Syncing to target...\n", len(srcPartLines))

	plSuccess := 0
	for _, src := range srcPartLines {
		id, err := uuid.Parse(src.ID)
		if err != nil {
			log.Printf("Invalid part line ID: %s, error: %v", src.ID, err)
			continue
		}
		woID, err := uuid.Parse(src.WorkOrderID)
		if err != nil {
			log.Printf("Invalid work order ID for part line %s: %s, error: %v", src.ID, src.WorkOrderID, err)
			continue
		}

		var deletedAt *time.Time
		if src.DeletedAt.Valid {
			deletedAt = &src.DeletedAt.Time
		}

		tgt := TargetWorkOrderPartLineRM{
			ID:          id,
			WorkOrderID: woID,
			PartID:      parseUUID(src.PartID),
			Description: src.Description,
			Quantity:    src.Quantity,
			Unit:        src.Unit,
			ListPrice:   src.ListPrice,
			LineAmount:  src.LineAmount,
			CreatedAt:   src.CreatedAt,
			UpdatedAt:   src.UpdatedAt,
			DeletedAt:   deletedAt,
			SyncedAt:    now,
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync part line %s: %v", src.ID, err)
		} else {
			plSuccess++
		}
	}
	fmt.Printf("Part lines sync complete. Success: %d / %d\n", plSuccess, len(srcPartLines))
	fmt.Println("All replication backfill completed successfully!")
}
