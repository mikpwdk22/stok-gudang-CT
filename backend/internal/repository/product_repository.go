package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ctstock/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepository struct {
	pool *pgxpool.Pool
}

func NewProductRepository(pool *pgxpool.Pool) *ProductRepository {
	return &ProductRepository{pool: pool}
}

// List queries products with search, brand filter, and pagination
func (r *ProductRepository) List(ctx context.Context, filter models.ProductFilter) ([]models.Product, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("(name ILIKE $%d OR brand ILIKE $%d)", argIdx, argIdx))
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	if filter.Brand != "" {
		conditions = append(conditions, fmt.Sprintf("brand ILIKE $%d", argIdx))
		args = append(args, filter.Brand)
		argIdx++
	}

	if filter.StockCounted != nil {
		conditions = append(conditions, fmt.Sprintf("is_stock_counted = $%d", argIdx))
		args = append(args, *filter.StockCounted)
		argIdx++
	}

	if filter.IsActive != nil {
		conditions = append(conditions, fmt.Sprintf("is_active = $%d", argIdx))
		args = append(args, *filter.IsActive)
		argIdx++
	} else {
		conditions = append(conditions, "is_active = true")
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// 1. Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM products %s", whereClause)
	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count products: %w", err)
	}

	// 2. Fetch paginated records
	limit := 50
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	query := fmt.Sprintf(`
		SELECT id, name, brand, stock_unit,
		       price_1, price_2, price_cash,
		       raw_price_1, raw_price_2, raw_price_cash,
		       pricing_basis, current_stock, is_stock_counted,
		       min_stock_alert, is_active, created_at, updated_at
		FROM products
		%s
		ORDER BY name ASC, brand ASC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query products: %w", err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		err := rows.Scan(
			&p.ID,
			&p.Name,
			&p.Brand,
			&p.StockUnit,
			&p.Price1,
			&p.Price2,
			&p.PriceCash,
			&p.RawPrice1,
			&p.RawPrice2,
			&p.RawPriceCash,
			&p.PricingBasis,
			&p.CurrentStock,
			&p.IsStockCounted,
			&p.MinStockAlert,
			&p.IsActive,
			&p.CreatedAt,
			&p.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan product: %w", err)
		}
		products = append(products, p)
	}

	return products, total, nil
}

// GetByID returns a single product
func (r *ProductRepository) GetByID(ctx context.Context, id int) (*models.Product, error) {
	query := `
		SELECT id, name, brand, stock_unit,
		       price_1, price_2, price_cash,
		       raw_price_1, raw_price_2, raw_price_cash,
		       pricing_basis, current_stock, is_stock_counted,
		       min_stock_alert, is_active, created_at, updated_at
		FROM products
		WHERE id = $1
	`
	var p models.Product
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&p.ID,
		&p.Name,
		&p.Brand,
		&p.StockUnit,
		&p.Price1,
		&p.Price2,
		&p.PriceCash,
		&p.RawPrice1,
		&p.RawPrice2,
		&p.RawPriceCash,
		&p.PricingBasis,
		&p.CurrentStock,
		&p.IsStockCounted,
		&p.MinStockAlert,
		&p.IsActive,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query product by id: %w", err)
	}
	return &p, nil
}

// Create inserts a new product
func (r *ProductRepository) Create(ctx context.Context, p *models.Product) error {
	query := `
		INSERT INTO products (
			name, brand, stock_unit,
			price_1, price_2, price_cash,
			raw_price_1, raw_price_2, raw_price_cash,
			pricing_basis, current_stock, is_stock_counted,
			min_stock_alert, is_active
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
		)
		RETURNING id, created_at, updated_at
	`
	err := r.pool.QueryRow(ctx, query,
		p.Name, p.Brand, p.StockUnit,
		p.Price1, p.Price2, p.PriceCash,
		p.RawPrice1, p.RawPrice2, p.RawPriceCash,
		p.PricingBasis, p.CurrentStock, p.IsStockCounted,
		p.MinStockAlert, p.IsActive,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create product: %w", err)
	}
	return nil
}

// Update modifies an existing product
func (r *ProductRepository) Update(ctx context.Context, p *models.Product) error {
	query := `
		UPDATE products SET
			name = $1,
			brand = $2,
			stock_unit = $3,
			price_1 = $4,
			price_2 = $5,
			price_cash = $6,
			raw_price_1 = $7,
			raw_price_2 = $8,
			raw_price_cash = $9,
			pricing_basis = $10,
			min_stock_alert = $11,
			is_active = $12,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $13
	`
	cmdTag, err := r.pool.Exec(ctx, query,
		p.Name, p.Brand, p.StockUnit,
		p.Price1, p.Price2, p.PriceCash,
		p.RawPrice1, p.RawPrice2, p.RawPriceCash,
		p.PricingBasis, p.MinStockAlert, p.IsActive,
		p.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update product: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("product not found")
	}
	return nil
}

// Delete marks product as inactive (soft delete)
func (r *ProductRepository) Delete(ctx context.Context, id int) error {
	query := `UPDATE products SET is_active = false, updated_at = CURRENT_TIMESTAMP WHERE id = $1`
	cmdTag, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete product: %w", err)
	}
	if cmdTag.RowsAffected() == 0 {
		return errors.New("product not found")
	}
	return nil
}

