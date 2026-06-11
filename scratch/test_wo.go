package main

import (
	"fmt"
	"log"

	"erp-billing-service/internal/domain"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "postgresql://billing_user:Billing@123@192.168.0.26:5441/billing_db"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	woIDStr := "c858ae9a-aae2-415e-ad11-b9c8846de6d0"
	woID := uuid.MustParse(woIDStr)

	// Delete existing invoices for this work order to allow regeneration
	var invoices []domain.Invoice
	if err := db.Where("source_system = ? AND source_reference_id = ?", domain.SourceSystemFSM, woIDStr).Find(&invoices).Error; err == nil {
		for _, inv := range invoices {
			fmt.Printf("Deleting old invoice %s and its items...\n", inv.ID)
			db.Delete(&domain.InvoiceItem{}, "invoice_id = ?", inv.ID)
			db.Delete(&domain.Invoice{}, "id = ?", inv.ID)
		}
	}

	var wo domain.WorkOrderRM
	err = db.
		Preload("Customer").
		Preload("ServiceLines").
		Preload("PartLines").
		First(&wo, "id = ?", woID).Error
	if err != nil {
		log.Fatalf("failed to fetch work order: %v", err)
	}

	fmt.Printf("WorkOrder details:\n")
	fmt.Printf("  ID: %s\n", wo.ID)
	fmt.Printf("  Summary: %s\n", wo.Summary)
	if wo.ServiceCategoryID != nil {
		fmt.Printf("  ServiceCategoryID: %s\n", wo.ServiceCategoryID.String())
	} else {
		fmt.Printf("  ServiceCategoryID is NIL\n")
	}

	// Now try inserting invoice using domain model mapping
	invoiceID := uuid.New()
	invoice := &domain.Invoice{
		ID:             invoiceID,
		OrganizationID: wo.OrganizationID,
		CustomerID:     *wo.CustomerID,
		SourceSystem:   domain.SourceSystemFSM,
		SourceReferenceID: &woIDStr,
		Subject:        "AUTO GENERATED TEST INVOICE",
		Status:         domain.InvoiceStatusDraft,
		ServiceCategoryID: wo.ServiceCategoryID,
	}

	fmt.Printf("Inserting test invoice %s with ServiceCategoryID = %v...\n", invoiceID, invoice.ServiceCategoryID)
	if err := db.Create(invoice).Error; err != nil {
		log.Fatalf("failed to insert invoice: %v", err)
	}

	var inserted domain.Invoice
	if err := db.First(&inserted, "id = ?", invoiceID).Error; err != nil {
		log.Fatalf("failed to find inserted invoice: %v", err)
	}

	if inserted.ServiceCategoryID != nil {
		fmt.Printf("Retrieved inserted ServiceCategoryID: %s\n", inserted.ServiceCategoryID.String())
	} else {
		fmt.Printf("Retrieved inserted ServiceCategoryID is NIL\n")
	}
}
