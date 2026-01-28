#!/bin/bash

# Database Migration Script for Billing Service
# Run this after the Docker build completes

echo "🔧 Running Billing Service Database Migration..."

# Database connection details (update these if needed)
DB_HOST="localhost"
DB_PORT="5432"
DB_USER="postgres"
DB_NAME="billing_db"

# Run the migration
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -f migrations/001_add_source_tracking_and_metadata.sql

if [ $? -eq 0 ]; then
    echo "✅ Migration completed successfully!"
    echo ""
    echo "Changes applied:"
    echo "  - invoice_number is now nullable"
    echo "  - Added source_system, source_reference_id, pdf_path columns"
    echo "  - Added metadata JSONB column to invoice_items"
    echo "  - Created performance indexes"
    echo ""
    echo "Next steps:"
    echo "  1. Restart the billing service: docker compose up -d"
    echo "  2. Test invoice creation"
else
    echo "❌ Migration failed. Please check the error above."
    exit 1
fi
