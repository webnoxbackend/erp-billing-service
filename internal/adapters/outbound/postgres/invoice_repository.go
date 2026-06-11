package postgres

import (
	"context"
	"fmt"
	"time"

	"erp-billing-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InvoiceRepository struct {
	db *gorm.DB
}

func NewInvoiceRepository(db *gorm.DB) *InvoiceRepository {
	return &InvoiceRepository{db: db}
}

func (r *InvoiceRepository) Create(ctx context.Context, invoice *domain.Invoice) error {
	return r.db.WithContext(ctx).Create(invoice).Error
}

func (r *InvoiceRepository) Update(ctx context.Context, invoice *domain.Invoice) error {
	return r.db.WithContext(ctx).Save(invoice).Error
}

func (r *InvoiceRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Invoice, error) {
	var invoice domain.Invoice
	err := r.db.WithContext(ctx).Preload("Items").Preload("Payments").First(&invoice, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	r.populateInvoiceAddressFields(ctx, &invoice)
	return &invoice, nil
}

func (r *InvoiceRepository) List(ctx context.Context, filter map[string]interface{}) ([]domain.Invoice, error) {
	var invoices []domain.Invoice
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("Payments").
		Where(filter).
		Order("created_at desc").
		Find(&invoices).Error
	if err == nil {
		r.populateInvoicesAddressFields(ctx, invoices)
	}
	return invoices, err
}

func (r *InvoiceRepository) ListByModule(ctx context.Context, orgID uuid.UUID, sourceSystem domain.SourceSystem) ([]domain.Invoice, error) {
	var invoices []domain.Invoice
	err := r.db.WithContext(ctx).
		Preload("Items").
		Preload("Payments").
		Where("organization_id = ? AND source_system = ?", orgID, sourceSystem).
		Order("created_at desc").
		Find(&invoices).Error
	if err == nil {
		r.populateInvoicesAddressFields(ctx, invoices)
	}
	return invoices, err
}

func (r *InvoiceRepository) populateInvoiceAddressFields(ctx context.Context, invoice *domain.Invoice) {
	if invoice == nil {
		return
	}
	if invoice.BillingAddressID != nil {
		var addr domain.AddressReadOnly
		if err := r.db.WithContext(ctx).First(&addr, "id = ?", *invoice.BillingAddressID).Error; err == nil {
			invoice.BillingStreet = addr.Street1
			if addr.Street2 != "" {
				invoice.BillingStreet = addr.Street1 + ", " + addr.Street2
			}
			invoice.BillingCity = addr.City
			invoice.BillingState = addr.State
			invoice.BillingCode = addr.PostalCode
			invoice.BillingCountry = addr.Country
		}
	}
	if invoice.ShippingAddressID != nil {
		var addr domain.AddressReadOnly
		if err := r.db.WithContext(ctx).First(&addr, "id = ?", *invoice.ShippingAddressID).Error; err == nil {
			invoice.ShippingStreet = addr.Street1
			if addr.Street2 != "" {
				invoice.ShippingStreet = addr.Street1 + ", " + addr.Street2
			}
			invoice.ShippingCity = addr.City
			invoice.ShippingState = addr.State
			invoice.ShippingCode = addr.PostalCode
			invoice.ShippingCountry = addr.Country
		}
	}
	if invoice.ServiceAddressID != nil {
		var addr domain.AddressReadOnly
		if err := r.db.WithContext(ctx).First(&addr, "id = ?", *invoice.ServiceAddressID).Error; err == nil {
			invoice.ServiceStreet = addr.Street1
			if addr.Street2 != "" {
				invoice.ServiceStreet = addr.Street1 + ", " + addr.Street2
			}
			invoice.ServiceCity = addr.City
			invoice.ServiceState = addr.State
			invoice.ServiceCode = addr.PostalCode
			invoice.ServiceCountry = addr.Country
		}
	}
}

func (r *InvoiceRepository) populateInvoicesAddressFields(ctx context.Context, invoices []domain.Invoice) {
	if len(invoices) == 0 {
		return
	}
	
	// Collect all unique address IDs
	addressIDs := make([]uuid.UUID, 0)
	addressIDMap := make(map[uuid.UUID]bool)
	for _, inv := range invoices {
		if inv.BillingAddressID != nil {
			if !addressIDMap[*inv.BillingAddressID] {
				addressIDMap[*inv.BillingAddressID] = true
				addressIDs = append(addressIDs, *inv.BillingAddressID)
			}
		}
		if inv.ShippingAddressID != nil {
			if !addressIDMap[*inv.ShippingAddressID] {
				addressIDMap[*inv.ShippingAddressID] = true
				addressIDs = append(addressIDs, *inv.ShippingAddressID)
			}
		}
		if inv.ServiceAddressID != nil {
			if !addressIDMap[*inv.ServiceAddressID] {
				addressIDMap[*inv.ServiceAddressID] = true
				addressIDs = append(addressIDs, *inv.ServiceAddressID)
			}
		}
	}
	
	if len(addressIDs) == 0 {
		return
	}
	
	// Fetch all addresses in one query
	var addresses []domain.AddressReadOnly
	if err := r.db.WithContext(ctx).Where("id IN ?", addressIDs).Find(&addresses).Error; err != nil {
		return
	}
	
	// Map address ID to address struct
	addrMap := make(map[uuid.UUID]domain.AddressReadOnly)
	for _, addr := range addresses {
		addrMap[addr.ID] = addr
	}
	
	// Populate invoice fields
	for i := range invoices {
		inv := &invoices[i]
		if inv.BillingAddressID != nil {
			if addr, ok := addrMap[*inv.BillingAddressID]; ok {
				inv.BillingStreet = addr.Street1
				if addr.Street2 != "" {
					inv.BillingStreet = addr.Street1 + ", " + addr.Street2
				}
				inv.BillingCity = addr.City
				inv.BillingState = addr.State
				inv.BillingCode = addr.PostalCode
				inv.BillingCountry = addr.Country
			}
		}
		if inv.ShippingAddressID != nil {
			if addr, ok := addrMap[*inv.ShippingAddressID]; ok {
				inv.ShippingStreet = addr.Street1
				if addr.Street2 != "" {
					inv.ShippingStreet = addr.Street1 + ", " + addr.Street2
				}
				inv.ShippingCity = addr.City
				inv.ShippingState = addr.State
				inv.ShippingCode = addr.PostalCode
				inv.ShippingCountry = addr.Country
			}
		}
		if inv.ServiceAddressID != nil {
			if addr, ok := addrMap[*inv.ServiceAddressID]; ok {
				inv.ServiceStreet = addr.Street1
				if addr.Street2 != "" {
					inv.ServiceStreet = addr.Street1 + ", " + addr.Street2
				}
				inv.ServiceCity = addr.City
				inv.ServiceState = addr.State
				inv.ServiceCode = addr.PostalCode
				inv.ServiceCountry = addr.Country
			}
		}
	}
}

func (r *InvoiceRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// First delete all invoice items
		if err := tx.Delete(&domain.InvoiceItem{}, "invoice_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete invoice items: %w", err)
		}

		// Then delete all payments
		if err := tx.Delete(&domain.Payment{}, "invoice_id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete payments: %w", err)
		}

		// Finally delete the invoice
		if err := tx.Delete(&domain.Invoice{}, "id = ?", id).Error; err != nil {
			return fmt.Errorf("failed to delete invoice: %w", err)
		}

		return nil
	})
}

func (r *InvoiceRepository) GetNextInvoiceNumber(ctx context.Context, orgID uuid.UUID) (string, error) {
	year := time.Now().Year()
	prefix := fmt.Sprintf("INV-%d-", year)

	var invoiceNumbers []string
	err := r.db.WithContext(ctx).Model(&domain.Invoice{}).
		Where("organization_id = ? AND invoice_number LIKE ?", orgID, prefix+"%").
		Pluck("invoice_number", &invoiceNumbers).Error
	if err != nil {
		return "", err
	}

	maxSeq := 0
	for _, num := range invoiceNumbers {
		var y, seq int
		_, err := fmt.Sscanf(num, "INV-%d-%d", &y, &seq)
		if err == nil && seq > maxSeq {
			maxSeq = seq
		}
	}

	return fmt.Sprintf("INV-%d-%04d", year, maxSeq+1), nil
}

func (r *InvoiceRepository) ClearItems(ctx context.Context, invoiceID uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&domain.InvoiceItem{}, "invoice_id = ?", invoiceID).Error
}
