package database

import (
	"fmt"
	"log"
	"time"

	"example-service/internal/domain"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// InitGORM initializes GORM database connection
func InitGORM(connectionURL string) (*gorm.DB, error) {
	if connectionURL == "" {
		return nil, fmt.Errorf("database connection URL is required")
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
	// Auto migrate all models
	err := db.AutoMigrate(
		&domain.Example{},
		// Add more domain entities here as needed
	)
	if err != nil {
		return fmt.Errorf("failed to auto migrate: %w", err)
	}

	log.Println("Database migrations completed successfully")
	return nil
}

