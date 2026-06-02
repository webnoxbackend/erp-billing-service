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

	fmt.Println("=== Checking Replicated Work Order Tables ===")

	tables := []string{
		"work_orders_readonly",
		"work_order_service_lines_readonly",
		"work_order_part_lines_readonly",
	}

	for _, table := range tables {
		var count int64
		err := db.Table(table).Count(&count).Error
		if err != nil {
			fmt.Printf("Error counting rows in %s: %v\n", table, err)
			continue
		}
		fmt.Printf("Table: %-36s | Row Count: %d\n", table, count)
	}

	fmt.Println("\n=== Verifying work_orders_readonly columns ===")
	rows, err := db.Raw("SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'work_orders_readonly'").Rows()
	if err != nil {
		log.Fatalf("Failed to fetch column metadata: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var columnName, dataType string
		rows.Scan(&columnName, &dataType)
		fmt.Printf("- %-24s (%s)\n", columnName, dataType)
	}

	fmt.Println("\n=== Sample Replicated Work Order ===")
	var results []map[string]interface{}
	err = db.Table("work_orders_readonly").Order("created_at DESC").Limit(1).Find(&results).Error
	if err != nil {
		log.Fatalf("Failed to fetch sample work order: %v", err)
	}

	if len(results) > 0 {
		wo := results[0]
		fmt.Printf("Work Order ID: %v\n", wo["id"])
		fmt.Printf("  Summary:       %v\n", wo["summary"])
		fmt.Printf("  Status:        %v\n", wo["status"])
		fmt.Printf("  SubTotal:      %v\n", wo["sub_total"])
		fmt.Printf("  GrandTotal:    %v\n", wo["grand_total"])
		fmt.Printf("  Synced At:     %v\n", wo["synced_at"])
		fmt.Printf("  Created At:    %v\n", wo["created_at"])
		fmt.Printf("  Updated At:    %v\n", wo["updated_at"])

		var slResults []map[string]interface{}
		db.Table("work_order_service_lines_readonly").Where("work_order_id = ?", wo["id"]).Find(&slResults)
		fmt.Printf("\n  Service Lines (%d):\n", len(slResults))
		for _, sl := range slResults {
			fmt.Printf("    - ID: %v | Desc: %s | Qty: %v | ListPrice: %v\n", sl["id"], sl["description"], sl["quantity"], sl["list_price"])
		}

		var plResults []map[string]interface{}
		db.Table("work_order_part_lines_readonly").Where("work_order_id = ?", wo["id"]).Find(&plResults)
		fmt.Printf("\n  Part Lines (%d):\n", len(plResults))
		for _, pl := range plResults {
			fmt.Printf("    - ID: %v | Desc: %s | Qty: %v | ListPrice: %v\n", pl["id"], pl["description"], pl["quantity"], pl["list_price"])
		}
	} else {
		fmt.Println("No records found in work_orders_readonly.")
	}
}
