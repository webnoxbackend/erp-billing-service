package database

import (
	"fmt"
	"log"
	"strings"
	"time"

	"erp-billing-service/internal/domain"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitGORM initializes GORM database connection
func InitGORM(connectionURL string) (*gorm.DB, error) {
	if connectionURL == "" {
		return nil, fmt.Errorf("database connection URL is required")
	}

	// Clean connection URL: replace 'schema=' with 'search_path=' if present
	// Postgres doesn't recognize 'schema' as a connection parameter
	if strings.Contains(connectionURL, "schema=") {
		connectionURL = strings.ReplaceAll(connectionURL, "schema=", "search_path=")
	}

	db, err := gorm.Open(postgres.Open(connectionURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set search_path to public schema explicitly to ensure consistency
	if err := db.Exec("SET search_path TO public").Error; err != nil {
		log.Printf("Warning: Could not set search_path: %v", err)
	}

	// Get underlying sql.DB to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Test the connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Successfully connected to database with GORM")
	return db, nil
}

// AutoMigrate runs GORM auto migrations
func AutoMigrate(db *gorm.DB) error {
	// Check for type mismatch (bigint vs uuid) which is common when migrating from old templates
	var orgIDType string
	db.Raw(`
		SELECT udt_name FROM information_schema.columns 
		WHERE table_schema = CURRENT_SCHEMA() 
		AND table_name = 'invoices' 
		AND column_name = 'organization_id'
	`).Scan(&orgIDType)

	if orgIDType == "int8" || orgIDType == "bigint" {
		log.Println("Detected bigint columns in invoices table. Dropping tables to prepare for UUID migration...")
		db.Exec("DROP TABLE IF EXISTS invoice_items CASCADE")
		db.Exec("DROP TABLE IF EXISTS payments CASCADE")
		db.Exec("DROP TABLE IF EXISTS invoices CASCADE")
		db.Exec("DROP TABLE IF EXISTS customer_rms CASCADE")
		db.Exec("DROP TABLE IF EXISTS contact_rms CASCADE")
		db.Exec("DROP TABLE IF EXISTS item_rms CASCADE")
		db.Exec("DROP TABLE IF EXISTS work_order_part_line_rms CASCADE")
		db.Exec("DROP TABLE IF EXISTS work_order_service_line_rms CASCADE")
		db.Exec("DROP TABLE IF EXISTS work_order_rms CASCADE")
	}

	// Auto migrate all models
	err := db.AutoMigrate(
		&domain.Invoice{},
		&domain.InvoiceItem{},
		&domain.Payment{},
		&domain.CustomerRM{},
		&domain.ContactRM{},
		&domain.ItemRM{},
		&domain.InvoiceAuditLog{},
		&domain.WorkOrderRM{},
		&domain.WorkOrderServiceLineRM{},
		&domain.WorkOrderPartLineRM{},
	)
	if err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}
