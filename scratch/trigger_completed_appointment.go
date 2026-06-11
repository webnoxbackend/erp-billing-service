package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"erp-billing-service/internal/domain"

	shared_events "github.com/efs/shared-events"
	shared_kafka "github.com/efs/shared-kafka"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// Load environment variables from .env
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, relying on environment")
	}

	dsn := "postgresql://billing_user:Billing@123@192.168.0.26:5441/billing_db"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	woIDStr := "c858ae9a-aae2-415e-ad11-b9c8846de6d0"
	woID := uuid.MustParse(woIDStr)

	// 1. Delete existing invoices for this work order to allow regeneration
	var invoices []domain.Invoice
	if err := db.Where("source_system = ? AND source_reference_id = ?", domain.SourceSystemFSM, woIDStr).Find(&invoices).Error; err == nil {
		for _, inv := range invoices {
			fmt.Printf("Deleting old invoice %s and its items...\n", inv.ID)
			db.Delete(&domain.InvoiceItem{}, "invoice_id = ?", inv.ID)
			db.Delete(&domain.Invoice{}, "id = ?", inv.ID)
		}
	}

	// 2. Fetch the work order details to inspect it
	var wo domain.WorkOrderRM
	err = db.
		Preload("Customer").
		Preload("ServiceLines").
		Preload("PartLines").
		First(&wo, "id = ?", woID).Error
	if err != nil {
		log.Fatalf("failed to fetch work order: %v", err)
	}

	fmt.Printf("Fetched WorkOrder summary: %q\n", wo.Summary)
	if wo.ServiceCategoryID != nil {
		fmt.Printf("WorkOrder ServiceCategoryID: %s\n", wo.ServiceCategoryID.String())
	} else {
		fmt.Println("WorkOrder ServiceCategoryID is NIL")
	}

	// 3. Initialize Kafka Producer
	kafkaCfg := shared_kafka.LoadConfigFromEnv()
	producer, err := shared_kafka.NewProducer(kafkaCfg, nil)
	if err != nil {
		log.Fatalf("Failed to initialize Kafka producer: %v", err)
	}
	defer producer.Close()

	// 4. Construct AppointmentCompleted Event
	apptID := uuid.New().String()
	now := time.Now().UTC()
	actualStart := now.Add(-2 * time.Hour)
	actualEnd := now

	payload := shared_events.AppointmentPayload{
		AppointmentID:     apptID,
		OrganizationID:    wo.OrganizationID.String(),
		WorkOrderID:       wo.ID.String(),
		AppointmentNumber: "APT-TEST-INT-001",
		ScheduledDate:     now,
		Status:            "Completed",
		ActualStartTime:   &actualStart,
		ActualEndTime:     &actualEnd,
		Duration:          120,
		Notes:             "Completed scheduled maintenance successfully.",
	}

	metadata := shared_events.NewEventMetadata(
		shared_events.AppointmentCompleted,
		shared_events.AggregateAppointment,
		apptID,
	)

	data, err := shared_events.Marshal(metadata, payload)
	if err != nil {
		log.Fatalf("Failed to marshal event: %v", err)
	}

	// 5. Publish Event
	topic := shared_events.GetTopicForEventType(metadata.EventType)
	fmt.Printf("Publishing AppointmentCompleted event for work order %s to topic %s...\n", woIDStr, topic)
	ctx := context.Background()
	err = producer.Publish(ctx, topic, metadata.ID, data)
	if err != nil {
		log.Fatalf("Failed to publish event: %v", err)
	}

	fmt.Println("Event successfully published. Waiting 3 seconds for the billing service consumer to process it...")
	time.Sleep(3 * time.Second)

	// 6. Query DB to check if the invoice was auto-generated
	var newInvoice domain.Invoice
	err = db.
		Preload("Items").
		First(&newInvoice, "source_system = ? AND source_reference_id = ?", domain.SourceSystemFSM, woIDStr).Error
	if err != nil {
		log.Fatalf("Failed to find generated invoice in DB: %v (Auto-generation may have failed or lagged)", err)
	}

	fmt.Printf("\nSUCCESS! Generated Invoice ID: %s\n", newInvoice.ID)
	fmt.Printf("  Status: %s\n", newInvoice.Status)
	fmt.Printf("  Invoice Number: %v\n", newInvoice.InvoiceNumber)
	if newInvoice.ServiceCategoryID != nil {
		fmt.Printf("  ServiceCategoryID: %s (Matches WorkOrder Category!)\n", newInvoice.ServiceCategoryID.String())
	} else {
		fmt.Println("  ServiceCategoryID is NIL (Mapping failed!)")
	}

	fmt.Printf("  Number of Items: %d\n", len(newInvoice.Items))
	for _, item := range newInvoice.Items {
		fmt.Printf("    Item: %s (%s) - Type: %s, Price: %.2f\n", item.Name, item.ID, item.ItemType, item.UnitPrice)
	}
}
