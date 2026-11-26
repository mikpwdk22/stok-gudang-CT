-- 000004_create_stock_transactions_table.down.sql
DROP TABLE IF EXISTS stock_transaction_items;
DROP TABLE IF EXISTS stock_transactions;
DROP TYPE IF EXISTS transaction_type_enum;

