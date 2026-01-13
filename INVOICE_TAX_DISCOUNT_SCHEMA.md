# Invoice Tax and Discount Schema Documentation

## Overview
The billing service already has **Tax** and **Discount** columns fully implemented in the invoice services and parts tables. This document provides a comprehensive overview of the schema and implementation.

## Database Schema

### Invoice Items Table (`invoice_items`)

The `invoice_items` table contains the following columns for tax and discount:

```sql
Column      | Type          | Description
------------|---------------|------------------------------------------
discount    | numeric(15,2) | Discount amount applied to the line item
tax         | numeric(15,2) | Tax amount applied to the line item
```

### Complete Table Structure

```sql
CREATE TABLE invoice_items (
    id          UUID PRIMARY KEY,
    invoice_id  UUID REFERENCES invoices(id),
    item_id     UUID,
    item_type   VARCHAR(20) DEFAULT 'service',  -- 'service' or 'part'
    name        VARCHAR(255),
    description TEXT,
    quantity    NUMERIC(15,2),
    unit_price  NUMERIC(15,2),
    discount    NUMERIC(15,2),                  -- ✅ Discount field
    tax         NUMERIC(15,2),                  -- ✅ Tax field
    total       NUMERIC(15,2),
    created_at  TIMESTAMP WITH TIME ZONE,
    updated_at  TIMESTAMP WITH TIME ZONE
);
```

## Domain Model

### InvoiceItem Struct (`internal/domain/invoice.go`)

```go
type InvoiceItem struct {
    ID          uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
    InvoiceID   uuid.UUID `gorm:"type:uuid;index" json:"invoice_id"`
    ItemID      uuid.UUID `gorm:"type:uuid;index" json:"item_id"`
    ItemType    string    `gorm:"type:varchar(20);default:'service'" json:"item_type"`
    Name        string    `gorm:"type:varchar(255)" json:"name"`
    Description string    `gorm:"type:text" json:"description"`
    Quantity    float64   `gorm:"type:decimal(15,2)" json:"quantity"`
    UnitPrice   float64   `gorm:"type:decimal(15,2)" json:"unit_price"`
    Discount    float64   `gorm:"type:decimal(15,2)" json:"discount"`     // ✅ Discount
    Tax         float64   `gorm:"type:decimal(15,2)" json:"tax"`          // ✅ Tax
    Total       float64   `gorm:"type:decimal(15,2)" json:"total"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

## Data Transfer Objects (DTOs)

### CreateInvoiceItem DTO (`internal/application/dto/invoice_dto.go`)

```go
type CreateInvoiceItem struct {
    ItemID      uuid.UUID `json:"item_id" validate:"required"`
    ItemType    string    `json:"item_type"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Quantity    float64   `json:"quantity" validate:"required,gt=0"`
    UnitPrice   float64   `json:"unit_price" validate:"required,gte=0"`
    Discount    float64   `json:"discount"`    // ✅ Discount
    Tax         float64   `json:"tax"`         // ✅ Tax
}
```

### ItemResponse DTO

```go
type ItemResponse struct {
    ItemID      uuid.UUID `json:"item_id"`
    ItemType    string    `json:"item_type"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    Quantity    float64   `json:"quantity"`
    UnitPrice   float64   `json:"unit_price"`
    Discount    float64   `json:"discount"`    // ✅ Discount
    Tax         float64   `json:"tax"`         // ✅ Tax
    Total       float64   `json:"total"`
}
```

## API Usage

### Creating an Invoice with Tax and Discount

**Endpoint:** `POST /api/v1/invoices`

**Request Body Example:**

```json
{
  "subject": "Service Invoice #001",
  "customer_id": "123e4567-e89b-12d3-a456-426614174000",
  "invoice_date": "2026-01-12T00:00:00Z",
  "due_date": "2026-02-12T00:00:00Z",
  "items": [
    {
      "item_id": "987fcdeb-51a2-43d1-b456-426614174111",
      "item_type": "service",
      "name": "Oil Change Service",
      "description": "Full synthetic oil change",
      "quantity": 1,
      "unit_price": 100.00,
      "discount": 10.00,
      "tax": 9.00
    },
    {
      "item_id": "987fcdeb-51a2-43d1-b456-426614174222",
      "item_type": "part",
      "name": "Oil Filter",
      "description": "Premium oil filter",
      "quantity": 1,
      "unit_price": 25.00,
      "discount": 2.50,
      "tax": 2.25
    }
  ]
}
```

### Response Example

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "invoice_number": "INV-2026-001",
  "subject": "Service Invoice #001",
  "status": "draft",
  "sub_total": 125.00,
  "discount_total": 12.50,
  "tax_total": 11.25,
  "total_amount": 123.75,
  "items": [
    {
      "item_id": "987fcdeb-51a2-43d1-b456-426614174111",
      "item_type": "service",
      "name": "Oil Change Service",
      "quantity": 1,
      "unit_price": 100.00,
      "discount": 10.00,
      "tax": 9.00,
      "total": 99.00
    },
    {
      "item_id": "987fcdeb-51a2-43d1-b456-426614174222",
      "item_type": "part",
      "name": "Oil Filter",
      "quantity": 1,
      "unit_price": 25.00,
      "discount": 2.50,
      "tax": 2.25,
      "total": 24.75
    }
  ]
}
```

## Calculation Logic

The invoice totals are calculated as follows:

```
For each line item:
  Line Total = (Unit Price × Quantity) - Discount + Tax

Invoice Totals:
  Sub Total = Sum of (Unit Price × Quantity) for all items
  Discount Total = Sum of all item discounts
  Tax Total = Sum of all item taxes
  Total Amount = Sub Total - Discount Total + Tax Total + Adjustment
```

## Database Migration Status

✅ **Schema is up to date**

The database schema has been verified and contains:
- `discount` column: `numeric(15,2)`
- `tax` column: `numeric(15,2)`

Auto-migration is handled by GORM in `internal/database/database.go`:

```go
func AutoMigrate(db *gorm.DB) error {
    err := db.AutoMigrate(
        &domain.Invoice{},
        &domain.InvoiceItem{},
        &domain.Payment{},
        // ... other models
    )
    return err
}
```

## Frontend Integration

The frontend should send tax and discount values for each line item when creating or updating invoices. The fields are:

- `discount`: Numeric value representing the discount amount (not percentage)
- `tax`: Numeric value representing the tax amount (not percentage)

If you need percentage-based calculations, implement them in the frontend before sending to the API.

## Summary

✅ **Tax column** - Fully implemented in schema, domain model, and DTOs  
✅ **Discount column** - Fully implemented in schema, domain model, and DTOs  
✅ **Database schema** - Up to date with proper data types  
✅ **API endpoints** - Ready to accept tax and discount values  
✅ **Auto-migration** - Configured and working  

**No additional changes are required.** The tax and discount functionality is already fully implemented and ready to use.
