package main

import (
	"fmt"
	"log"
	"time"

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

	// Clean up any old invoices from previous test CRM conversions
	estimateIDStr := "da184115-36aa-497a-866d-578dc5ee3420"
	var invoices []domain.Invoice
	if err := db.Where("source_system = ? AND source_reference_id = ?", domain.SourceSystemCRM, estimateIDStr).Find(&invoices).Error; err == nil {
		for _, inv := range invoices {
			fmt.Printf("Deleting old CRM invoice %s and its items...\n", inv.ID)
			db.Delete(&domain.InvoiceItem{}, "invoice_id = ?", inv.ID)
			db.Delete(&domain.Invoice{}, "id = ?", inv.ID)
		}
	}

	orgID := uuid.MustParse("9806d3ee-e401-4fdd-b6ad-9a2b0da40b31")
	custID := uuid.MustParse("2ef8b3f1-ea85-4c01-812a-3ba6041ab60f")
	catID := uuid.MustParse("7ded7976-c285-4b5c-a26c-ab33533f60a6")

	// Set up dependencies
	// Note: We use GORM repository directly to create so we don't have to instantiate whole service
	invoiceID := uuid.New()
	invoice := &domain.Invoice{
		ID:                invoiceID,
		OrganizationID:    orgID,
		CustomerID:        custID,
		Subject:           "CRM Estimate Conversion Test",
		InvoiceNumber:     nil,
		SourceSystem:      domain.SourceSystemCRM,
		SourceReferenceID: &estimateIDStr,
		InvoiceDate:       time.Now().UTC(),
		DueDate:           time.Now().UTC().AddDate(0, 0, 30),
		Status:            domain.InvoiceStatusDraft,
		Currency:          "INR",
		ServiceCategoryID: &catID,
	}

	item1 := domain.InvoiceItem{
		ID:                uuid.New(),
		InvoiceID:         invoiceID,
		ItemID:            uuid.New(),
		ItemType:          "service",
		Name:              "Consultation",
		Description:       "General consulting",
		Quantity:          2.0,
		UnitPrice:         1500.0,
		Total:             3000.0,
		ServiceCategoryID: &catID,
	}

	invoice.Items = []domain.InvoiceItem{item1}
	invoice.TotalAmount = 3000.0
	invoice.BalanceAmount = 3000.0

	fmt.Printf("Inserting CRM converted invoice %s with ServiceCategoryID = %s...\n", invoiceID, catID)
	if err := db.Create(invoice).Error; err != nil {
		log.Fatalf("failed to insert invoice: %v", err)
	}

	var inserted domain.Invoice
	if err := db.Preload("Items").First(&inserted, "id = ?", invoiceID).Error; err != nil {
		log.Fatalf("failed to find inserted CRM invoice: %v", err)
	}

	if inserted.ServiceCategoryID != nil {
		fmt.Printf("SUCCESS: Retrieved ServiceCategoryID: %s\n", inserted.ServiceCategoryID.String())
	} else {
		fmt.Println("FAILURE: Retrieved ServiceCategoryID is NIL")
	}

	if len(inserted.Items) > 0 && inserted.Items[0].ServiceCategoryID != nil {
		fmt.Printf("SUCCESS: Retrieved Item ServiceCategoryID: %s\n", inserted.Items[0].ServiceCategoryID.String())
	} else {
		fmt.Println("FAILURE: Item ServiceCategoryID is NIL")
	}
}
