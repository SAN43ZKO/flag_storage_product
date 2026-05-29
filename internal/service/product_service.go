package service

import (
	"context"
	"errors"
	"log"
	"storage/internal/model"
	"storage/internal/repository"
	"strings"
)

var (
	ErrNameRequired     = errors.New("name is required")
	ErrSKURequired      = errors.New("sku is required")
	ErrQuantityNegative = errors.New("quantity cannot be negative")
	ErrProductNotFound  = errors.New("product not found")
)

type ProductService struct {
	repo *repository.ProductRepo
}

func NewProductService(repo *repository.ProductRepo) *ProductService {
	return &ProductService{repo: repo}
}

func (s *ProductService) Validate(req model.CreateProductRequest) error {
	if req.Quantity < 0 {
		return ErrQuantityNegative
	}
	return nil
}

func (s *ProductService) Create(ctx context.Context, req model.CreateProductRequest) (model.Product, error) {
	if err := s.Validate(req); err != nil {
		return model.Product{}, err
	}
	return s.repo.Create(ctx, req)
}

func (s *ProductService) GetByID(ctx context.Context, id int64) (model.Product, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return model.Product{}, ErrProductNotFound
	}
	return p, err
}

func (s *ProductService) List(ctx context.Context, search string) ([]model.Product, error) {
	return s.repo.List(ctx, search)
}

func (s *ProductService) Update(ctx context.Context, id int64, req model.CreateProductRequest) (model.Product, error) {
	if err := s.Validate(req); err != nil {
		return model.Product{}, err
	}
	p, err := s.repo.Update(ctx, id, req)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return model.Product{}, ErrProductNotFound
	}
	return p, err
}

func (s *ProductService) UpdateImage(ctx context.Context, id int64, filename string) error {
	log.Printf("UpdateImage service called: id=%d, filename=%q", id, filename)
	product, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	req := model.CreateProductRequest{
		Name:      product.Name,
		SKU:       product.SKU,
		Quantity:  product.Quantity,
		Category:  product.Category,
		Unit:      product.Unit,
		ImagePath: filename,
	}
	log.Printf("Calling Update with ImagePath=%q", filename)
	_, err = s.repo.Update(ctx, id, req)
	return err
}

func (s *ProductService) Delete(ctx context.Context, id int64) error {
	err := s.repo.Delete(ctx, id)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return ErrProductNotFound
	}
	return err
}

func (s *ProductService) GetStats(ctx context.Context) (map[string]int, error) {
	total, err := s.repo.Count(ctx)
	if err != nil {
		return nil, err
	}
	categories, err := s.repo.CountCategories(ctx)
	if err != nil {
		return nil, err
	}
	low, err := s.repo.LowStock(ctx, 10)
	if err != nil {
		return nil, err
	}
	return map[string]int{
		"total_products":   total,
		"total_categories": categories,
		"low_stock_count":  len(low),
	}, nil
}

func (s *ProductService) GetCategories(ctx context.Context) ([]string, error) {
	products, err := s.repo.List(ctx, "")
	if err != nil {
		return nil, err
	}
	cats := make(map[string]struct{})
	for _, p := range products {
		if p.Category != nil && *p.Category != "" {
			cats[*p.Category] = struct{}{}
		}
	}
	var res []string
	for c := range cats {
		res = append(res, c)
	}
	return res, nil
}

func (s *ProductService) GetLowStock(ctx context.Context, threshold int) ([]model.Product, error) {
	return s.repo.LowStock(ctx, threshold)
}
