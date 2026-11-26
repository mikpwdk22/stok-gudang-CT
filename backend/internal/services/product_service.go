package services

import (
	"context"
	"errors"
	"strings"

	"ctstock/backend/internal/models"
	"ctstock/backend/internal/repository"
)

type ProductService struct {
	repo *repository.ProductRepository
}

func NewProductService(repo *repository.ProductRepository) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) ListProducts(ctx context.Context, filter models.ProductFilter) (*models.ProductListResponse, error) {
	products, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if products == nil {
		products = []models.Product{}
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	return &models.ProductListResponse{
		Data:       products,
		TotalCount: total,
		Limit:      limit,
		Offset:     filter.Offset,
	}, nil
}

func (s *ProductService) GetProductByID(ctx context.Context, id int) (*models.Product, error) {
	if id <= 0 {
		return nil, errors.New("invalid product id")
	}
	return s.repo.GetByID(ctx, id)
}

func (s *ProductService) CreateProduct(ctx context.Context, p *models.Product) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("nama barang (product name) is required")
	}

	if p.Brand != nil {
		trimmed := strings.TrimSpace(*p.Brand)
		if trimmed == "" {
			p.Brand = nil
		} else {
			p.Brand = &trimmed
		}
	}

	if p.StockUnit != nil {
		trimmed := strings.TrimSpace(*p.StockUnit)
		if trimmed == "" {
			p.StockUnit = nil
		} else {
			p.StockUnit = &trimmed
		}
	}

	if p.MinStockAlert < 0 {
		p.MinStockAlert = 5
	}
	p.IsActive = true

	// If stock wasn't explicitly initialized, keep it as uncounted (NULL)
	if p.CurrentStock != nil {
		p.IsStockCounted = true
	} else {
		p.IsStockCounted = false
	}

	return s.repo.Create(ctx, p)
}

func (s *ProductService) UpdateProduct(ctx context.Context, p *models.Product) error {
	if p.ID <= 0 {
		return errors.New("invalid product id")
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New("nama barang (product name) is required")
	}

	if p.Brand != nil {
		trimmed := strings.TrimSpace(*p.Brand)
		if trimmed == "" {
			p.Brand = nil
		} else {
			p.Brand = &trimmed
		}
	}

	if p.StockUnit != nil {
		trimmed := strings.TrimSpace(*p.StockUnit)
		if trimmed == "" {
			p.StockUnit = nil
		} else {
			p.StockUnit = &trimmed
		}
	}

	return s.repo.Update(ctx, p)
}

func (s *ProductService) DeleteProduct(ctx context.Context, id int) error {
	if id <= 0 {
		return errors.New("invalid product id")
	}
	return s.repo.Delete(ctx, id)
}

