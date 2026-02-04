-- Migration: Add complete item fields to item_rms table
-- This migration enhances the item_rms table to be a complete read-only replica
-- with inventory tracking, pricing, and unit information

-- Add new columns to item_rms table
ALTER TABLE item_rms
    -- Rename price to selling_price for clarity
    RENAME COLUMN price TO selling_price;

ALTER TABLE item_rms
    -- Add SKU index if not exists
    ADD COLUMN IF NOT EXISTS status VARCHAR(50) DEFAULT 'active',
    
    -- Pricing Information
    ADD COLUMN IF NOT EXISTS cost_price DECIMAL(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS currency VARCHAR(10) DEFAULT 'INR',
    
    -- Unit Information
    ADD COLUMN IF NOT EXISTS unit VARCHAR(50),
    
    -- Inventory Information (for goods/parts only)
    ADD COLUMN IF NOT EXISTS quantity_on_hand DECIMAL(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS quantity_available DECIMAL(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS quantity_reserved DECIMAL(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS quantity_damaged DECIMAL(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS reorder_level DECIMAL(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS reorder_quantity DECIMAL(15,2) DEFAULT 0,
    ADD COLUMN IF NOT EXISTS track_inventory BOOLEAN DEFAULT false,
    
    -- Tax Information
    ADD COLUMN IF NOT EXISTS taxable BOOLEAN DEFAULT false,
    ADD COLUMN IF NOT EXISTS tax_rate DECIMAL(5,2) DEFAULT 0;

-- Create index on SKU for faster lookups
CREATE INDEX IF NOT EXISTS idx_item_rms_sku ON item_rms(sku);

-- Create index on status for filtering
CREATE INDEX IF NOT EXISTS idx_item_rms_status ON item_rms(status);

-- Create index on item_type for filtering
CREATE INDEX IF NOT EXISTS idx_item_rms_item_type ON item_rms(item_type);

-- Update comment on table
COMMENT ON TABLE item_rms IS 'Complete read-only replica of items for sales order and inventory management';
