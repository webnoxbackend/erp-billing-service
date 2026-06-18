package main

import (
	"fmt"
	"log"
	"os"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("\nQuerying WorkOrderRM GORM model directly:")
	var wo domain.WorkOrderRM
	woID := uuid.MustParse("f470c748-62f4-4202-aafa-83ebbc1d6501")
	err = db.Debug().First(&wo, "id = ?", woID).Error
	if err != nil {
		fmt.Println("Error querying WorkOrderRM:", err)
	} else {
		fmt.Printf("SUCCESS! GORM Found WO ID: %s, Summary: %s, CustomerID: %v, OrgID: %s\n", wo.ID, wo.Summary, wo.CustomerID, wo.OrganizationID)
	}
}
