-- Migration: Add source tracking, PDF path, and metadata support to invoices
-- Date: 2026-01-13

-- Add source tracking fields to invoices table
ALTER TABLE invoices
ADD COLUMN IF NOT EXISTS source_system VARCHAR(20) DEFAULT 'MANUAL',
ADD COLUMN IF NOT EXISTS source_reference_id VARCHAR(100),
ADD COLUMN IF NOT EXISTS pdf_path VARCHAR(500);

-- Make invoice_number nullable (generated only on SEND, not creation)
ALTER TABLE invoices ALTER COLUMN invoice_number DROP NOT NULL;

-- Add metadata JSONB field to invoice_items table
ALTER TABLE invoice_items ADD COLUMN IF NOT EXISTS metadata JSONB;

-- Add indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_invoices_source_system ON invoices (source_system);

CREATE INDEX IF NOT EXISTS idx_invoices_status ON invoices (status);

CREATE INDEX IF NOT EXISTS idx_invoices_organization_status ON invoices (organization_id, status);

-- Add comment for documentation
COMMENT ON COLUMN invoices.source_system IS 'Origin module: FSM, CRM, INVENTORY, or MANUAL';

COMMENT ON COLUMN invoices.source_reference_id IS 'Opaque reference to originating module entity (e.g., WO-12345, DEAL-789)';

COMMENT ON COLUMN invoices.pdf_path IS 'File path to generated PDF, populated when invoice is sent';

COMMENT ON COLUMN invoice_items.metadata IS 'Module-specific metadata stored as JSONB (e.g., technician_id for FSM, deal_id for CRM)';