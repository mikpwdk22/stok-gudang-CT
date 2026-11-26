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

type TransactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

// Pool exposes the connection pool for atomic transactions
func (r *TransactionRepository) Pool() *pgxpool.Pool {
	return r.pool
}

// List queries transactions with filters and pagination
func (r *TransactionRepository) List(ctx context.Context, filter models.TransactionFilter) ([]models.Transaction, int, error) {
	var conditions []string
	var args []interface{}
	argIdx := 1

	if filter.TransactionType != nil {
		conditions = append(conditions, fmt.Sprintf("st.transaction_type = $%d", argIdx))
		args = append(args, *filter.TransactionType)
		argIdx++
	}

	if filter.UserID != nil {
		conditions = append(conditions, fmt.Sprintf("st.user_id = $%d", argIdx))
		args = append(args, *filter.UserID)
		argIdx++
	}

	if filter.ProductID != nil {
		conditions = append(conditions, fmt.Sprintf("EXISTS (SELECT 1 FROM stock_transaction_items WHERE transaction_id = st.id AND product_id = $%d)", argIdx))
		args = append(args, *filter.ProductID)
		argIdx++
	}

	if filter.FromDate != nil {
		conditions = append(conditions, fmt.Sprintf("st.transaction_date >= $%d", argIdx))
		args = append(args, *filter.FromDate)
		argIdx++
	}

	if filter.ToDate != nil {
		conditions = append(conditions, fmt.Sprintf("st.transaction_date <= $%d", argIdx))
		args = append(args, *filter.ToDate)
		argIdx++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM stock_transactions st
		%s
	`, whereClause)

	var total int
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count transactions: %w", err)
	}

	limit := 50
	if filter.Limit > 0 {
		limit = filter.Limit
	}
	offset := 0
	if filter.Offset > 0 {
		offset = filter.Offset
	}

	query := fmt.Sprintf(`
		SELECT st.id, st.transaction_type, st.user_id, u.full_name, st.notes, st.transaction_date, st.created_at
		FROM stock_transactions st
		JOIN users u ON st.user_id = u.id
		%s
		ORDER BY st.transaction_date DESC, st.id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)

	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var txs []models.Transaction
	var txIDs []int

	for rows.Next() {
		var t models.Transaction
		err := rows.Scan(
			&t.ID,
			&t.TransactionType,
			&t.UserID,
			&t.UserFullName,
			&t.Notes,
			&t.TransactionDate,
			&t.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan transaction: %w", err)
		}
		t.Items = []models.TransactionItem{}
		txs = append(txs, t)
		txIDs = append(txIDs, t.ID)
	}

	// Fetch items for all retrieved transactions in a single batch
	if len(txIDs) > 0 {
		itemsQuery := `
			SELECT sti.id, sti.transaction_id, sti.product_id, p.name, p.brand, sti.quantity, sti.unit_snapshot, sti.created_at
			FROM stock_transaction_items sti
			JOIN products p ON sti.product_id = p.id
			WHERE sti.transaction_id = ANY($1)
			ORDER BY sti.id ASC
		`
		itemRows, err := r.pool.Query(ctx, itemsQuery, txIDs)
		if err == nil {
			defer itemRows.Close()
			itemMap := make(map[int][]models.TransactionItem)
			for itemRows.Next() {
				var item models.TransactionItem
				if err := itemRows.Scan(
					&item.ID,
					&item.TransactionID,
					&item.ProductID,
					&item.ProductName,
					&item.ProductBrand,
					&item.Quantity,
					&item.UnitSnapshot,
					&item.CreatedAt,
				); err == nil {
					itemMap[item.TransactionID] = append(itemMap[item.TransactionID], item)
				}
			}
			for i := range txs {
				if items, ok := itemMap[txs[i].ID]; ok {
					txs[i].Items = items
				}
			}
		}
	}

	return txs, total, nil
}

// GetByID returns a single transaction with all its line items
func (r *TransactionRepository) GetByID(ctx context.Context, id int) (*models.Transaction, error) {
	query := `
		SELECT st.id, st.transaction_type, st.user_id, u.full_name, st.notes, st.transaction_date, st.created_at
		FROM stock_transactions st
		JOIN users u ON st.user_id = u.id
		WHERE st.id = $1
	`
	var t models.Transaction
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&t.ID,
		&t.TransactionType,
		&t.UserID,
		&t.UserFullName,
		&t.Notes,
		&t.TransactionDate,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to query transaction by id: %w", err)
	}

	// Fetch items
	itemsQuery := `
		SELECT sti.id, sti.transaction_id, sti.product_id, p.name, p.brand, sti.quantity, sti.unit_snapshot, sti.created_at
		FROM stock_transaction_items sti
		JOIN products p ON sti.product_id = p.id
		WHERE sti.transaction_id = $1
		ORDER BY sti.id ASC
	`
	itemRows, err := r.pool.Query(ctx, itemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction items: %w", err)
	}
	defer itemRows.Close()

	t.Items = []models.TransactionItem{}
	for itemRows.Next() {
		var item models.TransactionItem
		err := itemRows.Scan(
			&item.ID,
			&item.TransactionID,
			&item.ProductID,
			&item.ProductName,
			&item.ProductBrand,
			&item.Quantity,
			&item.UnitSnapshot,
			&item.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan transaction item: %w", err)
		}
		t.Items = append(t.Items, item)
	}

	return &t, nil
}

