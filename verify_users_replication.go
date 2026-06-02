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

	fmt.Println("=== Checking Old users Table Status ===")
	var oldUsersTableExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'users'
		)
	`).Scan(&oldUsersTableExists).Error
	if err != nil {
		log.Printf("Error checking for old users table: %v\n", err)
	} else {
		fmt.Printf("Old 'users' table exists: %t (Expected: false)\n", oldUsersTableExists)
	}

	fmt.Println("\n=== Checking Replicated users_readonly Table Status ===")
	var readonlyUsersTableExists bool
	err = db.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = 'users_readonly'
		)
	`).Scan(&readonlyUsersTableExists).Error
	if err != nil {
		log.Printf("Error checking for users_readonly table: %v\n", err)
	} else {
		fmt.Printf("Replicated 'users_readonly' table exists: %t (Expected: true)\n", readonlyUsersTableExists)
	}

	if readonlyUsersTableExists {
		var count int64
		err = db.Table("users_readonly").Count(&count).Error
		if err != nil {
			log.Fatalf("Failed to count rows in users_readonly: %v", err)
		}
		fmt.Printf("Row count in users_readonly: %d\n", count)

		fmt.Println("\n=== Verifying users_readonly columns ===")
		rows, err := db.Raw(`
			SELECT column_name, data_type, character_maximum_length, is_nullable 
			FROM information_schema.columns 
			WHERE table_name = 'users_readonly'
			ORDER BY column_name
		`).Rows()
		if err != nil {
			log.Fatalf("Failed to fetch column metadata: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var columnName, dataType, isNullable string
			var charMaxLen *int
			rows.Scan(&columnName, &dataType, &charMaxLen, &isNullable)
			if charMaxLen != nil {
				fmt.Printf("- %-20s (%s(%d)) | Nullable: %s\n", columnName, dataType, *charMaxLen, isNullable)
			} else {
				fmt.Printf("- %-20s (%s) | Nullable: %s\n", columnName, dataType, isNullable)
			}
		}

		fmt.Println("\n=== Sample Replicated Users ===")
		var sampleUsers []map[string]interface{}
		err = db.Table("users_readonly").Order("created_at DESC").Limit(3).Find(&sampleUsers).Error
		if err != nil {
			log.Fatalf("Failed to fetch sample users: %v", err)
		}

		for idx, user := range sampleUsers {
			fmt.Printf("[%d] User:\n", idx+1)
			fmt.Printf("  ID:                %v\n", user["id"])
			fmt.Printf("  Email:             %v\n", user["email"])
			fmt.Printf("  First Name:        %v\n", user["first_name"])
			fmt.Printf("  Last Name:         %v\n", user["last_name"])
			fmt.Printf("  Full Name:         %v\n", user["full_name"])
			fmt.Printf("  Role:              %v\n", user["role"])
			fmt.Printf("  User Type:         %v\n", user["user_type"])
			fmt.Printf("  Organization Name: %v\n", user["organization_name"])
			fmt.Printf("  Is Active:         %v\n", user["is_active"])
			fmt.Printf("  Synced At:         %v\n", user["synced_at"])
			fmt.Printf("  Created At:        %v\n", user["created_at"])
			fmt.Printf("  Updated At:        %v\n", user["updated_at"])
			fmt.Println()
		}
	} else {
		fmt.Println("Table 'users_readonly' was NOT found in the billing database.")
	}
}
