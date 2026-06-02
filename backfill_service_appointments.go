package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SourceServiceAppointment matches the schema of the source service_appointments table in efswomdbdev
type SourceServiceAppointment struct {
	ID                   string         `gorm:"type:uuid;primaryKey"`
	OrganizationID       string         `gorm:"type:varchar(50)"`
	WorkOrderID          string         `gorm:"type:uuid"`
	CustomerID           *string        `gorm:"type:uuid"`
	AppointmentNumber    string         `gorm:"size:50"`
	Subject              string         `gorm:"size:255"`
	ScheduledDate        time.Time      `gorm:"type:timestamp"`
	ScheduledTime        string         `gorm:"size:50"`
	ScheduledStartTime   *time.Time     `gorm:"type:timestamp"`
	ScheduledEndTime     *time.Time     `gorm:"type:timestamp"`
	Duration             int            `gorm:"type:integer"`
	Status               string         `gorm:"size:50"`
	ActualStartTime      *time.Time     `gorm:"type:timestamp"`
	ActualEndTime        *time.Time     `gorm:"type:timestamp"`
	StartLatitude        *float64       `gorm:"type:decimal(9,6)"`
	StartLongitude       *float64       `gorm:"type:decimal(9,6)"`
	EndLatitude          *float64       `gorm:"type:decimal(9,6)"`
	EndLongitude         *float64       `gorm:"type:decimal(9,6)"`
	AssignedTechnicianID *string        `gorm:"type:uuid"`
	AssignedCrewID       *string        `gorm:"type:uuid"`
	ServiceAddressID     *string        `gorm:"type:uuid"`
	BillingAddressID     *string        `gorm:"type:uuid"`
	Notes                string         `gorm:"type:text"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

func (SourceServiceAppointment) TableName() string {
	return "service_appointments"
}

// SourceServiceAppointmentResource matches the schema of the source service_appointment_resources table in efswomdbdev
type SourceServiceAppointmentResource struct {
	ID                   string         `gorm:"type:uuid;primaryKey"`
	OrganizationID       string         `gorm:"type:varchar(50)"`
	ServiceAppointmentID string         `gorm:"type:uuid"`
	ResourceType         string         `gorm:"size:50"`
	ResourceID           string         `gorm:"type:uuid"`
	StartTime            *time.Time     `gorm:"type:timestamp"`
	EndTime              *time.Time     `gorm:"type:timestamp"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index"`
}

func (SourceServiceAppointmentResource) TableName() string {
	return "service_appointment_resources"
}

// TargetServiceAppointmentRM matches target service_appointments_readonly
type TargetServiceAppointmentRM struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID       uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	WorkOrderID          uuid.UUID  `gorm:"type:uuid;index" json:"work_order_id"`
	CustomerID           *uuid.UUID `gorm:"type:uuid;index" json:"customer_id,omitempty"`
	AppointmentNumber    string     `gorm:"size:50;index" json:"appointment_number"`
	Subject              string     `gorm:"size:255" json:"subject,omitempty"`
	ScheduledDate        time.Time  `json:"scheduled_date"`
	ScheduledTime        string     `gorm:"size:50" json:"scheduled_time,omitempty"`
	ScheduledStartTime   *time.Time `json:"scheduled_start_time,omitempty"`
	ScheduledEndTime     *time.Time `json:"scheduled_end_time,omitempty"`
	Duration             int        `json:"duration"`
	Status               string     `gorm:"size:50;default:'SCHEDULED'" json:"status"`
	ActualStartTime      *time.Time `json:"actual_start_time,omitempty"`
	ActualEndTime        *time.Time `json:"actual_end_time,omitempty"`
	StartLatitude        *float64   `gorm:"type:decimal(9,6)" json:"start_latitude,omitempty"`
	StartLongitude       *float64   `gorm:"type:decimal(9,6)" json:"start_longitude,omitempty"`
	EndLatitude          *float64   `gorm:"type:decimal(9,6)" json:"end_latitude,omitempty"`
	EndLongitude         *float64   `gorm:"type:decimal(9,6)" json:"end_longitude,omitempty"`
	AssignedTechnicianID *uuid.UUID `gorm:"type:uuid" json:"assigned_technician_id,omitempty"`
	AssignedCrewID       *uuid.UUID `gorm:"type:uuid" json:"assigned_crew_id,omitempty"`
	ServiceAddressID     *uuid.UUID `gorm:"type:uuid;index" json:"service_address_id,omitempty"`
	BillingAddressID     *uuid.UUID `gorm:"type:uuid;index" json:"billing_address_id,omitempty"`
	Notes                string     `gorm:"type:text" json:"notes,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt             time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetServiceAppointmentRM) TableName() string {
	return "service_appointments_readonly"
}

// TargetServiceAppointmentResourceRM matches target service_appointment_resources_readonly
type TargetServiceAppointmentResourceRM struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID       uuid.UUID  `gorm:"type:uuid;index" json:"organization_id"`
	ServiceAppointmentID uuid.UUID  `gorm:"type:uuid;not null;index" json:"service_appointment_id"`
	ResourceType         string     `gorm:"size:50;not null" json:"resource_type"`
	ResourceID           uuid.UUID  `gorm:"type:uuid;not null;index" json:"resource_id"`
	StartTime            *time.Time `json:"start_time,omitempty"`
	EndTime              *time.Time `json:"end_time,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `gorm:"index" json:"deleted_at,omitempty"`
	SyncedAt             time.Time  `gorm:"autoUpdateTime" json:"synced_at"`
}

func (TargetServiceAppointmentResourceRM) TableName() string {
	return "service_appointment_resources_readonly"
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

	fmt.Println("Auto-migrating target read-only tables...")
	err = tgtDB.AutoMigrate(&TargetServiceAppointmentRM{}, &TargetServiceAppointmentResourceRM{})
	if err != nil {
		log.Fatalf("Failed to auto-migrate target tables: %v", err)
	}

	// 1. Migrate Service Appointments
	fmt.Println("Fetching all service appointments from source...")
	var srcAppointments []SourceServiceAppointment
	err = srcDB.Unscoped().Find(&srcAppointments).Error
	if err != nil {
		log.Fatalf("Failed to fetch source service appointments: %v", err)
	}
	fmt.Printf("Found %d service appointments in source. Syncing to target...\n", len(srcAppointments))

	now := time.Now().UTC()
	apptSuccess := 0
	for _, src := range srcAppointments {
		id, err := uuid.Parse(src.ID)
		if err != nil {
			log.Printf("Invalid source appointment ID: %s, error: %v", src.ID, err)
			continue
		}
		orgID, err := uuid.Parse(src.OrganizationID)
		if err != nil {
			log.Printf("Invalid organization ID: %s, error: %v", src.OrganizationID, err)
			continue
		}
		woID, err := uuid.Parse(src.WorkOrderID)
		if err != nil {
			log.Printf("Invalid work order ID: %s, error: %v", src.WorkOrderID, err)
			continue
		}

		var deletedAt *time.Time
		if src.DeletedAt.Valid {
			deletedAt = &src.DeletedAt.Time
		}

		tgt := TargetServiceAppointmentRM{
			ID:                   id,
			OrganizationID:       orgID,
			WorkOrderID:          woID,
			CustomerID:           parseUUID(src.CustomerID),
			AppointmentNumber:    src.AppointmentNumber,
			Subject:              src.Subject,
			ScheduledDate:        src.ScheduledDate,
			ScheduledTime:        src.ScheduledTime,
			ScheduledStartTime:   src.ScheduledStartTime,
			ScheduledEndTime:     src.ScheduledEndTime,
			Duration:             src.Duration,
			Status:               src.Status,
			ActualStartTime:      src.ActualStartTime,
			ActualEndTime:        src.ActualEndTime,
			StartLatitude:        src.StartLatitude,
			StartLongitude:       src.StartLongitude,
			EndLatitude:          src.EndLatitude,
			EndLongitude:         src.EndLongitude,
			AssignedTechnicianID: parseUUID(src.AssignedTechnicianID),
			AssignedCrewID:       parseUUID(src.AssignedCrewID),
			ServiceAddressID:     parseUUID(src.ServiceAddressID),
			BillingAddressID:     parseUUID(src.BillingAddressID),
			Notes:                src.Notes,
			CreatedAt:            src.CreatedAt,
			UpdatedAt:            src.UpdatedAt,
			DeletedAt:            deletedAt,
			SyncedAt:             now,
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync service appointment %s: %v", src.ID, err)
		} else {
			apptSuccess++
		}
	}
	fmt.Printf("Service appointments sync complete. Success: %d / %d\n", apptSuccess, len(srcAppointments))

	// 2. Migrate Assigned Resources
	fmt.Println("Fetching all service appointment resources from source...")
	var srcResources []SourceServiceAppointmentResource
	err = srcDB.Unscoped().Find(&srcResources).Error
	if err != nil {
		log.Fatalf("Failed to fetch source service appointment resources: %v", err)
	}
	fmt.Printf("Found %d service appointment resources in source. Syncing to target...\n", len(srcResources))

	resSuccess := 0
	for _, src := range srcResources {
		id, err := uuid.Parse(src.ID)
		if err != nil {
			log.Printf("Invalid source resource assignment ID: %s, error: %v", src.ID, err)
			continue
		}
		orgID, err := uuid.Parse(src.OrganizationID)
		if err != nil {
			log.Printf("Invalid organization ID: %s, error: %v", src.OrganizationID, err)
			continue
		}
		apptID, err := uuid.Parse(src.ServiceAppointmentID)
		if err != nil {
			log.Printf("Invalid service appointment ID: %s, error: %v", src.ServiceAppointmentID, err)
			continue
		}
		resourceID, err := uuid.Parse(src.ResourceID)
		if err != nil {
			log.Printf("Invalid resource ID: %s, error: %v", src.ResourceID, err)
			continue
		}

		var deletedAt *time.Time
		if src.DeletedAt.Valid {
			deletedAt = &src.DeletedAt.Time
		}

		tgt := TargetServiceAppointmentResourceRM{
			ID:                   id,
			OrganizationID:       orgID,
			ServiceAppointmentID: apptID,
			ResourceType:         src.ResourceType,
			ResourceID:           resourceID,
			StartTime:            src.StartTime,
			EndTime:              src.EndTime,
			CreatedAt:            src.CreatedAt,
			UpdatedAt:            src.UpdatedAt,
			DeletedAt:            deletedAt,
			SyncedAt:             now,
		}

		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync service appointment resource %s: %v", src.ID, err)
		} else {
			resSuccess++
		}
	}
	fmt.Printf("Service appointment resources sync complete. Success: %d / %d\n", resSuccess, len(srcResources))
	fmt.Println("All replication backfill completed successfully!")
}
