-- Migration: Add delivered_date to sales_orders table
-- Date: 2026-07-22

ALTER TABLE sales_orders ADD COLUMN IF NOT EXISTS delivered_date TIMESTAMP;
COMMENT ON COLUMN sales_orders.delivered_date IS 'Timestamp when the sales order was delivered to the customer';
