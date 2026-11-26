-- 000003_create_products_table.up.sql
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    brand VARCHAR(100),
    stock_unit VARCHAR(30),                  -- Inventory unit: 'pcs', 'meter', 'rol', 'dus', 'kg', etc. (NULL if unverified)

    -- Normalized Decimal Selling Prices (NULL if not set)
    price_1 DECIMAL(15, 2),                  -- Harga 1: Normal selling price
    price_2 DECIMAL(15, 2),                  -- Harga 2: Negotiated price
    price_cash DECIMAL(15, 2),               -- Cash: Physical cash price

    -- Original Raw Values from Excel (Preserves exact raw text)
    raw_price_1 VARCHAR(100),                -- e.g. "10000", "6.000/Lb"
    raw_price_2 VARCHAR(100),                -- e.g. "9500"
    raw_price_cash VARCHAR(100),             -- e.g. "9000", "240/pcs"

    -- Measurement / Pricing Basis (Independent from stock_unit)
    pricing_basis VARCHAR(50),               -- e.g. '/Kg', '/Lb', '/psg', '/mm', '/m³', '/pcs'

    -- Inventory Control
    current_stock NUMERIC(15, 3) CHECK (current_stock IS NULL OR current_stock >= 0),
    is_stock_counted BOOLEAN DEFAULT FALSE,  -- FALSE = "Belum dihitung", TRUE = Stock is counted
    min_stock_alert NUMERIC(15, 3) DEFAULT 5, -- Alert threshold

    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

-- Unique index per product variant (same name can exist under different brands)
CREATE UNIQUE INDEX IF NOT EXISTS idx_products_name_brand ON products (name, COALESCE(brand, ''));
CREATE INDEX IF NOT EXISTS idx_products_name ON products(name);
CREATE INDEX IF NOT EXISTS idx_products_brand ON products(brand);
CREATE INDEX IF NOT EXISTS idx_products_stock_counted ON products(is_stock_counted);
CREATE INDEX IF NOT EXISTS idx_products_is_active ON products(is_active);

