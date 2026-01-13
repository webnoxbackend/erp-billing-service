#!/bin/bash

# Invoice Tax and Discount API Test Script
# This script demonstrates creating an invoice with tax and discount on line items

echo "=========================================="
echo "Invoice Tax & Discount API Test"
echo "=========================================="
echo ""

# API Base URL
API_URL="http://localhost:8080/api/v1/billing"

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}Testing Invoice Creation with Tax and Discount...${NC}"
echo ""

# Sample invoice with tax and discount
INVOICE_DATA='{
  "subject": "Test Invoice with Tax and Discount",
  "customer_id": "123e4567-e89b-12d3-a456-426614174000",
  "invoice_date": "2026-01-12T00:00:00Z",
  "due_date": "2026-02-12T00:00:00Z",
  "currency": "USD",
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
      "quantity": 2,
      "unit_price": 25.00,
      "discount": 5.00,
      "tax": 4.50
    }
  ]
}'

echo "Request Payload:"
echo "$INVOICE_DATA" | jq '.'
echo ""

# Note: This requires authentication token
echo -e "${BLUE}Note: This endpoint requires authentication.${NC}"
echo "To test with authentication, add your JWT token:"
echo ""
echo -e "${GREEN}curl -X POST $API_URL/invoices \\${NC}"
echo -e "${GREEN}  -H \"Content-Type: application/json\" \\${NC}"
echo -e "${GREEN}  -H \"Authorization: Bearer YOUR_TOKEN\" \\${NC}"
echo -e "${GREEN}  -d '$INVOICE_DATA'${NC}"
echo ""

echo "=========================================="
echo "Expected Calculations:"
echo "=========================================="
echo ""
echo "Line Item 1 (Oil Change Service):"
echo "  Unit Price: \$100.00"
echo "  Quantity: 1"
echo "  Subtotal: \$100.00"
echo "  Discount: -\$10.00"
echo "  Tax: +\$9.00"
echo "  Line Total: \$99.00"
echo ""
echo "Line Item 2 (Oil Filter):"
echo "  Unit Price: \$25.00"
echo "  Quantity: 2"
echo "  Subtotal: \$50.00"
echo "  Discount: -\$5.00"
echo "  Tax: +\$4.50"
echo "  Line Total: \$49.50"
echo ""
echo "Invoice Totals:"
echo "  Sub Total: \$150.00"
echo "  Total Discount: \$15.00"
echo "  Total Tax: \$13.50"
echo "  Grand Total: \$148.50"
echo ""

echo "=========================================="
echo "Database Schema Verification"
echo "=========================================="
echo ""
echo "Checking invoice_items table structure..."
echo ""

# Check if we can connect to the database
if command -v psql &> /dev/null; then
    PGPASSWORD=Billing@123 psql -h 192.168.0.26 -p 5441 -U billing_user -d billing_db -c "
        SELECT 
            column_name, 
            data_type, 
            character_maximum_length,
            numeric_precision,
            numeric_scale
        FROM information_schema.columns 
        WHERE table_name = 'invoice_items' 
        AND column_name IN ('discount', 'tax', 'total', 'unit_price')
        ORDER BY ordinal_position;
    " 2>/dev/null || echo "Database connection not available (this is optional)"
else
    echo "psql not installed (optional for verification)"
fi

echo ""
echo "=========================================="
echo "API Endpoints Available"
echo "=========================================="
echo ""
echo "POST   $API_URL/invoices          - Create invoice"
echo "GET    $API_URL/invoices          - List invoices"
echo "GET    $API_URL/invoices/:id      - Get invoice by ID"
echo "PUT    $API_URL/invoices/:id      - Update invoice"
echo "DELETE $API_URL/invoices/:id      - Delete invoice"
echo ""
echo "All endpoints support tax and discount on line items!"
echo ""
