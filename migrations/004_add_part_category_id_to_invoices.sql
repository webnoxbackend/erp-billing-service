-- Migration: Add Part Category ID to invoices and invoice_items
-- Date: 2026-07-11

-- Add part_category_id to invoices table
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS part_category_id UUID;
CREATE INDEX IF NOT EXISTS idx_invoices_part_category ON invoices (part_category_id);

-- Add part_category_id to invoice_items table
ALTER TABLE invoice_items ADD COLUMN IF NOT EXISTS part_category_id UUID;
CREATE INDEX IF NOT EXISTS idx_invoice_items_part_category ON invoice_items (part_category_id);
