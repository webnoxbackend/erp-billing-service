-- Migration: Add Sales Order and Sales Return support
-- Date: 2026-01-20

-- Create sales_orders table
CREATE TABLE IF NOT EXISTS sales_orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id UUID NOT NULL,
    customer_id UUID NOT NULL,
    contact_id UUID,
    order_number VARCHAR(50) UNIQUE,
    order_date TIMESTAMP NOT NULL,
    status VARCHAR(20) DEFAULT 'draft',
    sub_total DECIMAL(15, 2) NOT NULL,
    discount_total DECIMAL(15, 2) DEFAULT 0,
    tax_total DECIMAL(15, 2) DEFAULT 0,
    tds_amount DECIMAL(15, 2) DEFAULT 0,
    tcs_amount DECIMAL(15, 2) DEFAULT 0,
    total_amount DECIMAL(15, 2) NOT NULL,
    invoice_id UUID,
    shipped_date TIMESTAMP,
    terms TEXT,
    notes TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP
);

-- Create sales_order_items table
CREATE TABLE IF NOT EXISTS sales_order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    sales_order_id UUID NOT NULL,
    item_id UUID NOT NULL,
    item_type VARCHAR(20) DEFAULT 'product',
    name VARCHAR(255) NOT NULL,
    description TEXT,
    quantity DECIMAL(15, 2) NOT NULL,
    unit_price DECIMAL(15, 2) NOT NULL,
    discount DECIMAL(15, 2) DEFAULT 0,
    tax DECIMAL(15, 2) DEFAULT 0,
    total DECIMAL(15, 2) NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT fk_sales_order FOREIGN KEY (sales_order_id) REFERENCES sales_orders (id) ON DELETE CASCADE
);

-- Create sales_returns table
CREATE TABLE IF NOT EXISTS sales_returns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    organization_id UUID NOT NULL,
    sales_order_id UUID NOT NULL,
    return_number VARCHAR(50) UNIQUE,
    return_date TIMESTAMP NOT NULL,
    status VARCHAR(20) DEFAULT 'draft',
    return_amount DECIMAL(15, 2) NOT NULL,
    return_reason TEXT,
    notes TEXT,
    approved_date TIMESTAMP,
    received_date TIMESTAMP,
    receiving_notes TEXT,
    refunded_date TIMESTAMP,
    refund_payment_id UUID,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    deleted_at TIMESTAMP,
    CONSTRAINT fk_sales_order_return FOREIGN KEY (sales_order_id) REFERENCES sales_orders (id)
);

-- Create sales_return_items table
CREATE TABLE IF NOT EXISTS sales_return_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid (),
    sales_return_id UUID NOT NULL,
    sales_order_item_id UUID NOT NULL,
    returned_quantity DECIMAL(15, 2) NOT NULL,
    unit_price DECIMAL(15, 2) NOT NULL,
    tax DECIMAL(15, 2) DEFAULT 0,
    total DECIMAL(15, 2) NOT NULL,
    reason TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    CONSTRAINT fk_sales_return FOREIGN KEY (sales_return_id) REFERENCES sales_returns (id) ON DELETE CASCADE,
    CONSTRAINT fk_sales_order_item FOREIGN KEY (sales_order_item_id) REFERENCES sales_order_items (id)
);

-- Add indexes for sales_orders
CREATE INDEX IF NOT EXISTS idx_sales_orders_org_status ON sales_orders (organization_id, status);

CREATE INDEX IF NOT EXISTS idx_sales_orders_customer ON sales_orders (customer_id);

CREATE INDEX IF NOT EXISTS idx_sales_orders_invoice ON sales_orders (invoice_id);

CREATE INDEX IF NOT EXISTS idx_sales_orders_order_number ON sales_orders (order_number);

-- Add indexes for sales_order_items
CREATE INDEX IF NOT EXISTS idx_sales_order_items_order ON sales_order_items (sales_order_id);

CREATE INDEX IF NOT EXISTS idx_sales_order_items_item ON sales_order_items (item_id);

-- Add indexes for sales_returns
CREATE INDEX IF NOT EXISTS idx_sales_returns_org_status ON sales_returns (organization_id, status);

CREATE INDEX IF NOT EXISTS idx_sales_returns_order ON sales_returns (sales_order_id);

CREATE INDEX IF NOT EXISTS idx_sales_returns_return_number ON sales_returns (return_number);

CREATE INDEX IF NOT EXISTS idx_sales_returns_refund_payment ON sales_returns (refund_payment_id);

-- Add indexes for sales_return_items
CREATE INDEX IF NOT EXISTS idx_sales_return_items_return ON sales_return_items (sales_return_id);

CREATE INDEX IF NOT EXISTS idx_sales_return_items_order_item ON sales_return_items (sales_order_item_id);

-- Add sales_order_id to invoices table
ALTER TABLE invoices ADD COLUMN IF NOT EXISTS sales_order_id UUID;

ALTER TABLE invoices
ADD COLUMN IF NOT EXISTS tds_amount DECIMAL(15, 2) DEFAULT 0;

ALTER TABLE invoices
ADD COLUMN IF NOT EXISTS tcs_amount DECIMAL(15, 2) DEFAULT 0;

-- Add index for invoice sales_order_id
CREATE INDEX IF NOT EXISTS idx_invoices_sales_order ON invoices (sales_order_id);

-- Add foreign key constraint for invoice to sales_order (if not exists)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_invoice_sales_order'
    ) THEN
        ALTER TABLE invoices ADD CONSTRAINT fk_invoice_sales_order 
        FOREIGN KEY (sales_order_id) REFERENCES sales_orders(id);

END IF;

END $$;

-- Add foreign key constraint for sales_order to invoice (if not exists)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_sales_order_invoice'
    ) THEN
        ALTER TABLE sales_orders ADD CONSTRAINT fk_sales_order_invoice 
        FOREIGN KEY (invoice_id) REFERENCES invoices(id);
    END IF;
END $$;

-- Add payment_type and sales_return_id to payments table
ALTER TABLE payments
ADD COLUMN IF NOT EXISTS payment_type VARCHAR(20) DEFAULT 'payment';

ALTER TABLE payments ADD COLUMN IF NOT EXISTS sales_return_id UUID;

-- Add index for payment sales_return_id
CREATE INDEX IF NOT EXISTS idx_payments_sales_return ON payments (sales_return_id);

-- Add foreign key constraint for payment to sales_return (if not exists)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_payment_sales_return'
    ) THEN
        ALTER TABLE payments ADD CONSTRAINT fk_payment_sales_return 
        FOREIGN KEY (sales_return_id) REFERENCES sales_returns(id);

END IF;

END $$;

-- Add foreign key constraint for sales_return to payment (if not exists)
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_sales_return_payment'
    ) THEN
        ALTER TABLE sales_returns ADD CONSTRAINT fk_sales_return_payment 
        FOREIGN KEY (refund_payment_id) REFERENCES payments(id);
    END IF;
END $$;

-- Add comments for documentation
COMMENT ON
TABLE sales_orders IS 'Customer sales orders for inventory module';

COMMENT ON TABLE sales_order_items IS 'Line items for sales orders';

COMMENT ON
TABLE sales_returns IS 'Sales returns for paid and shipped orders';

COMMENT ON
TABLE sales_return_items IS 'Line items for sales returns';

COMMENT ON COLUMN sales_orders.order_number IS 'Generated order number (format: SO-YYYYMMDD-XXXX)';

COMMENT ON COLUMN sales_orders.status IS 'Order status: draft, confirmed, invoiced, partially_paid, paid, shipped, completed, cancelled';

COMMENT ON COLUMN sales_orders.tds_amount IS 'Tax Deducted at Source amount';

COMMENT ON COLUMN sales_orders.tcs_amount IS 'Tax Collected at Source amount';

COMMENT ON COLUMN sales_returns.return_number IS 'Generated return number (format: RMA-YYYYMMDD-XXXX)';

COMMENT ON COLUMN sales_returns.status IS 'Return status: draft, approved, received, refunded';

COMMENT ON COLUMN invoices.sales_order_id IS 'Reference to originating sales order (if created from sales order)';

COMMENT ON COLUMN invoices.tds_amount IS 'Tax Deducted at Source amount';

COMMENT ON COLUMN invoices.tcs_amount IS 'Tax Collected at Source amount';

COMMENT ON COLUMN payments.payment_type IS 'Payment type: payment or refund';

COMMENT ON COLUMN payments.sales_return_id IS 'Reference to sales return (for refund payments)';