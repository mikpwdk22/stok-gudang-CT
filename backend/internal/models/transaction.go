package models

import "time"

// TransactionType defines valid inventory movement types
type TransactionType string

const (
	TransactionTypeOpeningStock    TransactionType = "opening_stock"
	TransactionTypeStockIn         TransactionType = "stock_in"
	TransactionTypeStockOut        TransactionType = "stock_out"
	TransactionTypeStockAdjustment TransactionType = "stock_adjustment"
)

// Transaction represents the header of an inventory movement event
type Transaction struct {
	ID              int               `json:"id"`
	TransactionType TransactionType   `json:"transaction_type"`
	UserID          int               `json:"user_id"`
	UserFullName    string            `json:"user_full_name,omitempty"`
	Notes           *string           `json:"notes"`
	TransactionDate time.Time         `json:"transaction_date"`
	CreatedAt       time.Time         `json:"created_at"`
	Items           []TransactionItem `json:"items,omitempty"`
}

// TransactionItem represents a line item in an inventory movement
type TransactionItem struct {
	ID            int       `json:"id"`
	TransactionID int       `json:"transaction_id"`
	ProductID     int       `json:"product_id"`
	ProductName   string    `json:"product_name,omitempty"`
	ProductBrand  *string   `json:"product_brand,omitempty"`
	Quantity      float64   `json:"quantity"`
	UnitSnapshot  string    `json:"unit_snapshot"`
	CreatedAt     time.Time `json:"created_at"`
}

// StockMovementItemRequest represents an individual item in a Stock In / Stock Out request
type StockMovementItemRequest struct {
	ProductID    int     `json:"product_id"`
	Quantity     float64 `json:"quantity"`
	UnitSnapshot string  `json:"unit_snapshot,omitempty"`
}

// StockMovementRequest represents a batch Stock In / Stock Out payload
type StockMovementRequest struct {
	Notes *string                    `json:"notes"`
	Items []StockMovementItemRequest `json:"items"`
}

// TransactionFilter defines search parameters for querying transaction history
type TransactionFilter struct {
	TransactionType *TransactionType
	UserID          *int
	ProductID       *int
	FromDate        *time.Time
	ToDate          *time.Time
	Limit           int
	Offset          int
}

// TransactionListResponse wraps paginated transactions
type TransactionListResponse struct {
	Data       []Transaction `json:"data"`
	TotalCount int           `json:"total_count"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
}

