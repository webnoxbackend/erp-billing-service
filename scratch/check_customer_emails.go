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

	var wo domain.WorkOrderRM
	err = db.First(&wo, "id = ?", woID).Error
	if err != nil {
		log.Fatalf("failed to fetch work order: %v", err)
	}

	fmt.Printf("WorkOrder details:\n")
	fmt.Printf("  CustomerID: %v\n", wo.CustomerID)
	fmt.Printf("  ContactID: %v\n", wo.ContactID)

	if wo.CustomerID != nil {
		var customer domain.CustomerRM
		if err := db.First(&customer, "id = ?", *wo.CustomerID).Error; err == nil {
			fmt.Printf("Customer details:\n")
			fmt.Printf("  ID: %s\n", customer.ID)
			fmt.Printf("  DisplayName: %s\n", customer.DisplayName)
			fmt.Printf("  Email: %q\n", customer.Email)
		} else {
			fmt.Printf("Failed to fetch customer: %v\n", err)
		}
	}

	if wo.ContactID != nil {
		var contact domain.ContactRM
		if err := db.First(&contact, "id = ?", *wo.ContactID).Error; err == nil {
			fmt.Printf("Contact details:\n")
			fmt.Printf("  ID: %s\n", contact.ID)
			fmt.Printf("  Email: %q\n", contact.Email)
		} else {
			fmt.Printf("Failed to fetch contact: %v\n", err)
		}
	}
}
