package models

import "time"

// Product represents a catalog product in the store
type Product struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`
	Brand          *string   `json:"brand"`
	StockUnit      *string   `json:"stock_unit"`
	Price1         *float64  `json:"price_1"`
	Price2         *float64  `json:"price_2"`
	PriceCash      *float64  `json:"price_cash"`
	RawPrice1      *string   `json:"raw_price_1"`
	RawPrice2      *string   `json:"raw_price_2"`
	RawPriceCash   *string   `json:"raw_price_cash"`
	PricingBasis   *string   `json:"pricing_basis"`
	CurrentStock   *float64  `json:"current_stock"`
	IsStockCounted bool      `json:"is_stock_counted"`
	MinStockAlert  float64   `json:"min_stock_alert"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ProductFilter defines search and pagination parameters
type ProductFilter struct {
	Search       string
	Brand        string
	StockCounted *bool
	IsActive     *bool
	Limit        int
	Offset       int
}

// ProductListResponse wraps paginated products with metadata
type ProductListResponse struct {
	Data       []Product `json:"data"`
	TotalCount int       `json:"total_count"`
	Limit      int       `json:"limit"`
	Offset     int       `json:"offset"`
}

