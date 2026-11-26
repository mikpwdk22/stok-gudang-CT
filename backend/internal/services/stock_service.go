package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"ctstock/backend/internal/models"
	"ctstock/backend/internal/repository"
)

type StockService struct {
	txRepo *repository.TransactionRepository
}

func NewStockService(txRepo *repository.TransactionRepository) *StockService {
	return &StockService{txRepo: txRepo}
}

// StockIn processes incoming inventory atomically
func (s *StockService) StockIn(ctx context.Context, userID int, req models.StockMovementRequest) (*models.Transaction, error) {
	if len(req.Items) == 0 {
		return nil, errors.New("at least one item is required for stock in")
	}

	for _, item := range req.Items {
		if item.ProductID <= 0 {
			return nil, errors.New("invalid product_id")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("quantity must be greater than zero")
		}
	}

	tx, err := s.txRepo.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin atomic transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Create Stock Transaction Header
	var txID int
	createTxSQL := `
		INSERT INTO stock_transactions (transaction_type, user_id, notes, transaction_date)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		RETURNING id
	`
	err = tx.QueryRow(ctx, createTxSQL, models.TransactionTypeStockIn, userID, req.Notes).Scan(&txID)
	if err != nil {
		return nil, fmt.Errorf("failed to record stock in header: %w", err)
	}

	// 2. Process each item: update stock & record line item
	var createdItems []models.TransactionItem
	for _, item := range req.Items {
		var currentStock *float64
		var isCounted bool
		var stockUnit *string
		var prodName string

		// Lock product row
		lockSQL := `
			SELECT name, stock_unit, current_stock, is_stock_counted
			FROM products
			WHERE id = $1 AND is_active = true
			FOR UPDATE
		`
		err := tx.QueryRow(ctx, lockSQL, item.ProductID).Scan(&prodName, &stockUnit, &currentStock, &isCounted)
		if err != nil {
			return nil, fmt.Errorf("product with id %d not found or inactive: %w", item.ProductID, err)
		}

		unitSnapshot := item.UnitSnapshot
		if unitSnapshot == "" {
			if stockUnit != nil && *stockUnit != "" {
				unitSnapshot = *stockUnit
			} else {
				unitSnapshot = "pcs"
			}
		}

		// Calculate new stock: If previously uncounted (NULL), base starts at 0 + incoming quantity
		newStock := item.Quantity
		if currentStock != nil {
			newStock = *currentStock + item.Quantity
		}

		// Update product
		updateSQL := `
			UPDATE products SET
				current_stock = $1,
				is_stock_counted = true,
				stock_unit = COALESCE(stock_unit, $2),
				updated_at = CURRENT_TIMESTAMP
			WHERE id = $3
		`
		_, err = tx.Exec(ctx, updateSQL, newStock, unitSnapshot, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("failed to update stock for product '%s': %w", prodName, err)
		}

		// Insert transaction line item
		var itemID int
		insertItemSQL := `
			INSERT INTO stock_transaction_items (transaction_id, product_id, quantity, unit_snapshot)
			VALUES ($1, $2, $3, $4)
			RETURNING id
		`
		err = tx.QueryRow(ctx, insertItemSQL, txID, item.ProductID, item.Quantity, unitSnapshot).Scan(&itemID)
		if err != nil {
			return nil, fmt.Errorf("failed to insert stock in line item: %w", err)
		}

		createdItems = append(createdItems, models.TransactionItem{
			ID:            itemID,
			TransactionID: txID,
			ProductID:     item.ProductID,
			ProductName:   prodName,
			Quantity:      item.Quantity,
			UnitSnapshot:  unitSnapshot,
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit stock in transaction: %w", err)
	}

	return s.txRepo.GetByID(ctx, txID)
}

// StockOut processes outgoing inventory with strict safeguards
func (s *StockService) StockOut(ctx context.Context, userID int, req models.StockMovementRequest) (*models.Transaction, error) {
	// 1. Mandatory Note Rule
	if req.Notes == nil || strings.TrimSpace(*req.Notes) == "" {
		return nil, errors.New("catatan (notes) wajib diisi untuk Stock Out (misal: 'Terjual ke Toko Subur' atau 'Dipakai toko')")
	}

	if len(req.Items) == 0 {
		return nil, errors.New("at least one item is required for stock out")
	}

	for _, item := range req.Items {
		if item.ProductID <= 0 {
			return nil, errors.New("invalid product_id")
		}
		if item.Quantity <= 0 {
			return nil, errors.New("quantity must be greater than zero")
		}
	}

	tx, err := s.txRepo.Pool().Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin atomic transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// 2. Create Stock Transaction Header
	var txID int
	createTxSQL := `
		INSERT INTO stock_transactions (transaction_type, user_id, notes, transaction_date)
		VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		RETURNING id
	`
	err = tx.QueryRow(ctx, createTxSQL, models.TransactionTypeStockOut, userID, req.Notes).Scan(&txID)
	if err != nil {
		return nil, fmt.Errorf("failed to record stock out header: %w", err)
	}

	// 3. Process each item with strict safeguards
	for _, item := range req.Items {
		var currentStock *float64
		var isCounted bool
		var stockUnit *string
		var prodName string

		// Lock product row to prevent race conditions
		lockSQL := `
			SELECT name, stock_unit, current_stock, is_stock_counted
			FROM products
			WHERE id = $1 AND is_active = true
			FOR UPDATE
		`
		err := tx.QueryRow(ctx, lockSQL, item.ProductID).Scan(&prodName, &stockUnit, &currentStock, &isCounted)
		if err != nil {
			return nil, fmt.Errorf("product with id %d not found or inactive: %w", item.ProductID, err)
		}

		// Safeguard 1: Block Stock Out if product is still "Belum dihitung"
		if !isCounted || currentStock == nil {
			return nil, fmt.Errorf("barang '%s' masih berstatus 'Belum dihitung'. Lakukan hitung fisik (Stock Opname) terlebih dahulu sebelum mengeluarkan barang", prodName)
		}

		// Safeguard 2: Prevent negative stock
		if *currentStock < item.Quantity {
			unitDisplay := "pcs"
			if stockUnit != nil {
				unitDisplay = *stockUnit
			}
			return nil, fmt.Errorf("stok tidak cukup untuk '%s'. Stok tersedia: %.2f %s, diminta: %.2f %s",
				prodName, *currentStock, unitDisplay, item.Quantity, unitDisplay)
		}

		unitSnapshot := item.UnitSnapshot
		if unitSnapshot == "" {
			if stockUnit != nil && *stockUnit != "" {
				unitSnapshot = *stockUnit
			} else {
				unitSnapshot = "pcs"
			}
		}

		newStock := *currentStock - item.Quantity

		// Update product current_stock
		updateSQL := `
			UPDATE products SET
				current_stock = $1,
				updated_at = CURRENT_TIMESTAMP
			WHERE id = $2
		`
		_, err = tx.Exec(ctx, updateSQL, newStock, item.ProductID)
		if err != nil {
			return nil, fmt.Errorf("failed to deduct stock for product '%s': %w", prodName, err)
		}

		// Insert transaction line item
		insertItemSQL := `
			INSERT INTO stock_transaction_items (transaction_id, product_id, quantity, unit_snapshot)
			VALUES ($1, $2, $3, $4)
		`
		_, err = tx.Exec(ctx, insertItemSQL, txID, item.ProductID, item.Quantity, unitSnapshot)
		if err != nil {
			return nil, fmt.Errorf("failed to insert stock out line item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit stock out transaction: %w", err)
	}

	return s.txRepo.GetByID(ctx, txID)
}

// ListTransactions retrieves transactions matching filters
func (s *StockService) ListTransactions(ctx context.Context, filter models.TransactionFilter) (*models.TransactionListResponse, error) {
	txs, total, err := s.txRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if txs == nil {
		txs = []models.Transaction{}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	return &models.TransactionListResponse{
		Data:       txs,
		TotalCount: total,
		Limit:      limit,
		Offset:     filter.Offset,
	}, nil
}

// GetTransactionByID retrieves a single transaction with items
func (s *StockService) GetTransactionByID(ctx context.Context, id int) (*models.Transaction, error) {
	if id <= 0 {
		return nil, errors.New("invalid transaction id")
	}
	return s.txRepo.GetByID(ctx, id)
}

