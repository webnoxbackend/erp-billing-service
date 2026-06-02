package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to billing DB: %v", err)
	}

	fmt.Println("Connecting to billing database to clean up old tables...")

	tablesToDrop := []string{
		"work_order_rms",
		"work_order_service_line_rms",
		"work_order_part_line_rms",
	}

	for _, table := range tablesToDrop {
		fmt.Printf("Dropping table %s (if exists) CASCADE...\n", table)
		err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table)).Error
		if err != nil {
			log.Printf("Error dropping table %s: %v\n", table, err)
		} else {
			fmt.Printf("Successfully dropped table %s (if it existed).\n", table)
		}
	}

	fmt.Println("Cleanup complete!")
}
