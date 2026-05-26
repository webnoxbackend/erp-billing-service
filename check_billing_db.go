package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "postgresql://billing_user:Billing@123@192.168.0.26:5441/billing_db"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	rows, err := db.Raw("SELECT column_name FROM information_schema.columns WHERE table_name = 'work_order_rms'").Rows()
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("Columns in work_order_rms (billing_db):")
	for rows.Next() {
		var columnName string
		rows.Scan(&columnName)
		fmt.Println("-", columnName)
	}

	fmt.Println("\nRecent work orders in work_order_rms:")
	var results []map[string]interface{}
	db.Table("work_order_rms").Order("updated_at DESC").Limit(5).Find(&results)
	for _, wo := range results {
		fmt.Printf("WO ID: %v\n  Summary: %v\n  RequestID: %v\n  EstimateID: %v\n  ServiceCategoryID: %v\n\n",
			wo["id"], wo["summary"], wo["request_id"], wo["estimate_id"], wo["service_category_id"])
	}
}
