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

	fmt.Println("=== Checking Replicated Service Appointment Tables ===")

	tables := []string{
		"service_appointments_readonly",
		"service_appointment_resources_readonly",
	}

	for _, table := range tables {
		var count int64
		err := db.Table(table).Count(&count).Error
		if err != nil {
			fmt.Printf("Error counting rows in %s: %v\n", table, err)
			continue
		}
		fmt.Printf("Table: %-42s | Row Count: %d\n", table, count)
	}

	fmt.Println("\n=== Verifying service_appointments_readonly Columns ===")
	rows, err := db.Raw("SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'service_appointments_readonly' ORDER BY ordinal_position").Rows()
	if err != nil {
		log.Fatalf("Failed to fetch columns: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var columnName, dataType string
		rows.Scan(&columnName, &dataType)
		fmt.Printf("- %-28s (%s)\n", columnName, dataType)
	}

	fmt.Println("\n=== Sample Replicated Service Appointment ===")
	var results []map[string]interface{}
	err = db.Table("service_appointments_readonly").Order("created_at DESC").Limit(1).Find(&results).Error
	if err != nil {
		log.Fatalf("Failed to fetch sample service appointment: %v", err)
	}

	if len(results) > 0 {
		sa := results[0]
		fmt.Printf("Appointment ID:    %v\n", sa["id"])
		fmt.Printf("  Number:          %v\n", sa["appointment_number"])
		fmt.Printf("  Subject:         %v\n", sa["subject"])
		fmt.Printf("  Status:          %v\n", sa["status"])
		fmt.Printf("  Scheduled Date:  %v\n", sa["scheduled_date"])
		fmt.Printf("  Duration:        %v mins\n", sa["duration"])
		fmt.Printf("  Assigned Tech:   %v\n", sa["assigned_technician_id"])
		fmt.Printf("  Assigned Crew:   %v\n", sa["assigned_crew_id"])
		fmt.Printf("  Synced At:       %v\n", sa["synced_at"])
		fmt.Printf("  Created At:      %v\n", sa["created_at"])
		fmt.Printf("  Updated At:      %v\n", sa["updated_at"])

		var resResults []map[string]interface{}
		db.Table("service_appointment_resources_readonly").Where("service_appointment_id = ?", sa["id"]).Find(&resResults)
		fmt.Printf("\n  Assigned Resources (%d):\n", len(resResults))
		for _, res := range resResults {
			fmt.Printf("    - Resource ID: %v | Type: %s | StartTime: %v | EndTime: %v\n", 
				res["resource_id"], res["resource_type"], res["start_time"], res["end_time"])
		}
	} else {
		fmt.Println("No records found in service_appointments_readonly.")
	}
}
