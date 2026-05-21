package service

import (
	"context"
	"errors"
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

func (s *ProductService) Delete(ctx context.Context, id int64) error {
	err := s.repo.Delete(ctx, id)
	if err != nil && strings.Contains(err.Error(), "not found") {
		return ErrProductNotFound
	}
	return err
}
