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

	contactID := "2c40e799-171a-4b63-be3b-73bed6231e09"
	err = db.Table("contact_rms").Where("id = ?", contactID).Update("email", "diveshwebnox@gmail.com").Error
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Contact email corrected successfully to diveshwebnox@gmail.com")
}
