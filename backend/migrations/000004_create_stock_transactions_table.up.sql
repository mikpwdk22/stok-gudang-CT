-- 000004_create_stock_transactions_table.up.sql
DO $$ BEGIN
    CREATE TYPE transaction_type_enum AS ENUM (
        'opening_stock',
        'stock_in',
        'stock_out',
        'stock_adjustment'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

CREATE TABLE IF NOT EXISTS stock_transactions (
    id SERIAL PRIMARY KEY,
    transaction_type transaction_type_enum NOT NULL,
    user_id INTEGER NOT NULL REFERENCES users(id),
    notes TEXT,
    transaction_date TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_transactions_type ON stock_transactions(transaction_type);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON stock_transactions(transaction_date);

CREATE TABLE IF NOT EXISTS stock_transaction_items (
    id SERIAL PRIMARY KEY,
    transaction_id INTEGER NOT NULL REFERENCES stock_transactions(id) ON DELETE CASCADE,
    product_id INTEGER NOT NULL REFERENCES products(id),
    quantity NUMERIC(15, 3) NOT NULL CHECK (quantity > 0),
    unit_snapshot VARCHAR(30) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_trans_items_tx ON stock_transaction_items(transaction_id);
CREATE INDEX IF NOT EXISTS idx_trans_items_product ON stock_transaction_items(product_id);

