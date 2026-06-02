package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// SourceCustomer matches the schema of the source customers table in efcusdev
type SourceCustomer struct {
	ID                string         `gorm:"type:uuid;primaryKey"`
	OrganizationID    string         `gorm:"type:uuid"`
	ExternalKey       *string        `gorm:"size:100"`
	CustomerType      string         `gorm:"size:20"`
	DisplayName       string         `gorm:"size:255"`
	CompanyName       string         `gorm:"size:255"`
	FirstName         string         `gorm:"size:100"`
	LastName          string         `gorm:"size:100"`
	Salutation        string         `gorm:"size:20"`
	Email             string         `gorm:"size:255"`
	PhoneWork         string         `gorm:"size:50"`
	PhoneMobile       string         `gorm:"size:50"`
	WebsiteURL        string         `gorm:"size:255"`
	TaxNumber         string         `gorm:"size:50"`
	CurrencyCode      string         `gorm:"size:10"`
	PaymentTerms      string         `gorm:"size:50"`
	IsTaxable         bool           `json:"is_taxable"`
	Industry          string         `gorm:"size:100"`
	Rating            string         `gorm:"size:50"`
	Ownership         string         `gorm:"size:50"`
	AnnualRevenue     float64        `gorm:"type:numeric(18,2)"`
	PortalEnabled     bool           `json:"portal_enabled"`
	Status            string         `gorm:"size:20"`
	SourceSystem      string         `gorm:"size:30"`
	SourceID          string         `gorm:"size:100"`
	ServiceAddressID  *string        `gorm:"type:uuid"`
	BillingAddressID  *string        `gorm:"type:uuid"`
	ShippingAddressID *string        `gorm:"type:uuid"`
	AccountOwner      string         `gorm:"size:255"`
	AccountSite       string         `gorm:"size:80"`
	ParentAccountID   *string        `gorm:"type:uuid"`
	CustomerLanguage  string         `gorm:"size:50"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index"`
}

func (SourceCustomer) TableName() string {
	return "customers"
}

// TargetCustomerRM matches domain.CustomerRM in erp-billing-service
type TargetCustomerRM struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey" json:"id"`
	OrganizationID    uuid.UUID      `gorm:"type:uuid;index" json:"organization_id"`
	ExternalKey       *string        `gorm:"size:100" json:"external_key"`
	CustomerType      string         `gorm:"size:20;index" json:"customer_type"`
	DisplayName       string         `gorm:"size:255;index" json:"display_name"`
	CompanyName       string         `gorm:"size:255" json:"company_name"`
	FirstName         string         `gorm:"size:100" json:"first_name"`
	LastName          string         `gorm:"size:100" json:"last_name"`
	Salutation        string         `gorm:"size:20" json:"salutation"`
	Email             string         `gorm:"size:255;index" json:"email"`
	PhoneWork         string         `gorm:"size:50" json:"phone_work"`
	PhoneMobile       string         `gorm:"size:50" json:"phone_mobile"`
	WebsiteURL        string         `gorm:"size:255" json:"website_url"`
	TaxNumber         string         `gorm:"size:50" json:"tax_number"`
	CurrencyCode      string         `gorm:"size:10;default:'INR'" json:"currency_code"`
	PaymentTerms      string         `gorm:"size:50" json:"payment_terms"`
	IsTaxable         bool           `json:"is_taxable"`
	Industry          string         `gorm:"size:100" json:"industry"`
	Rating            string         `gorm:"size:50" json:"rating"`
	Ownership         string         `gorm:"size:50" json:"ownership"`
	AnnualRevenue     float64        `gorm:"type:numeric(18,2)" json:"annual_revenue"`
	PortalEnabled     bool           `json:"portal_enabled"`
	Status            string         `gorm:"size:20;index" json:"status"`
	SourceSystem      string         `gorm:"size:30" json:"source_system"`
	SourceID          string         `gorm:"size:100" json:"source_id"`
	ServiceAddressID  *uuid.UUID     `gorm:"type:uuid;index" json:"service_address_id"`
	BillingAddressID  *uuid.UUID     `gorm:"type:uuid;index" json:"billing_address_id"`
	ShippingAddressID *uuid.UUID     `gorm:"type:uuid;index" json:"shipping_address_id"`
	AccountOwner      string         `gorm:"size:255" json:"account_owner"`
	AccountSite       string         `gorm:"size:80" json:"account_site"`
	ParentAccountID   *uuid.UUID     `gorm:"type:uuid;index" json:"parent_account_id"`
	CustomerLanguage  string         `gorm:"size:50;default:'English'" json:"customer_language"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

func (TargetCustomerRM) TableName() string {
	return "customers_readonly"
}

func main() {
	sourceDSN := "postgresql://efcusdev:efcusdev@123@192.168.0.26:5456/efcusdev"
	targetDSN := "postgresql://efsbillingdevdb:efsbillingdevdb@123@192.168.0.26:5467/efsbillingdevdb"

	fmt.Println("Connecting to source database...")
	srcDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to source DB: %v", err)
	}

	fmt.Println("Connecting to target database...")
	tgtDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to target DB: %v", err)
	}

	fmt.Println("Fetching all customers from source (including deleted)...")
	var srcCustomers []SourceCustomer
	err = srcDB.Unscoped().Find(&srcCustomers).Error
	if err != nil {
		log.Fatalf("Failed to fetch source customers: %v", err)
	}

	fmt.Printf("Found %d customers in source. Syncing to target...\n", len(srcCustomers))

	successCount := 0
	for _, src := range srcCustomers {
		srcID, err := uuid.Parse(src.ID)
		if err != nil {
			log.Printf("Invalid source ID: %s, error: %v", src.ID, err)
			continue
		}

		orgID, err := uuid.Parse(src.OrganizationID)
		if err != nil {
			log.Printf("Invalid organization ID for customer %s: %s, error: %v", src.ID, src.OrganizationID, err)
			continue
		}

		var serviceAddressID, billingAddressID, shippingAddressID, parentAccountID *uuid.UUID
		if src.ServiceAddressID != nil && *src.ServiceAddressID != "" {
			if u, err := uuid.Parse(*src.ServiceAddressID); err == nil {
				serviceAddressID = &u
			}
		}
		if src.BillingAddressID != nil && *src.BillingAddressID != "" {
			if u, err := uuid.Parse(*src.BillingAddressID); err == nil {
				billingAddressID = &u
			}
		}
		if src.ShippingAddressID != nil && *src.ShippingAddressID != "" {
			if u, err := uuid.Parse(*src.ShippingAddressID); err == nil {
				shippingAddressID = &u
			}
		}
		if src.ParentAccountID != nil && *src.ParentAccountID != "" {
			if u, err := uuid.Parse(*src.ParentAccountID); err == nil {
				parentAccountID = &u
			}
		}

		tgt := TargetCustomerRM{
			ID:                srcID,
			OrganizationID:    orgID,
			ExternalKey:       src.ExternalKey,
			CustomerType:      src.CustomerType,
			DisplayName:       src.DisplayName,
			CompanyName:       src.CompanyName,
			FirstName:         src.FirstName,
			LastName:          src.LastName,
			Salutation:        src.Salutation,
			Email:             src.Email,
			PhoneWork:         src.PhoneWork,
			PhoneMobile:       src.PhoneMobile,
			WebsiteURL:        src.WebsiteURL,
			TaxNumber:         src.TaxNumber,
			CurrencyCode:      src.CurrencyCode,
			PaymentTerms:      src.PaymentTerms,
			IsTaxable:         src.IsTaxable,
			Industry:          src.Industry,
			Rating:            src.Rating,
			Ownership:         src.Ownership,
			AnnualRevenue:     src.AnnualRevenue,
			PortalEnabled:     src.PortalEnabled,
			Status:            src.Status,
			SourceSystem:      src.SourceSystem,
			SourceID:          src.SourceID,
			ServiceAddressID:  serviceAddressID,
			BillingAddressID:  billingAddressID,
			ShippingAddressID: shippingAddressID,
			AccountOwner:      src.AccountOwner,
			AccountSite:       src.AccountSite,
			ParentAccountID:   parentAccountID,
			CustomerLanguage:  src.CustomerLanguage,
			CreatedAt:         src.CreatedAt,
			UpdatedAt:         src.UpdatedAt,
			DeletedAt:         src.DeletedAt,
		}

		// Ensure we bypass any GORM auto-timestamps and insert the exact source CreatedAt/UpdatedAt
		// We do this by executing a DB Save. Since target CreatedAt is not zero, GORM will insert it.
		// To be 100% sure we don't hit GORM default CreatedAt bypass, we can use raw insert if needed,
		// but let's try GORM Save or Create first.
		err = tgtDB.Session(&gorm.Session{CreateBatchSize: 1000}).Save(&tgt).Error
		if err != nil {
			log.Printf("Failed to sync customer %s: %v", src.ID, err)
		} else {
			successCount++
		}
	}

	fmt.Printf("Sync complete. Successfully synced %d / %d records.\n", successCount, len(srcCustomers))
}
