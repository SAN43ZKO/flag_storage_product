package service

import (
	"context"

	"storage/internal/model"
	"storage/internal/repository"
)

type ProductChangeService struct {
	changeRepo  *repository.ProductChangeRepo
	productRepo *repository.ProductRepo
}

func NewProductChangeService(changeRepo *repository.ProductChangeRepo, productRepo *repository.ProductRepo) *ProductChangeService {
	return &ProductChangeService{changeRepo: changeRepo, productRepo: productRepo}
}

func (s *ProductChangeService) List(ctx context.Context, search string) ([]model.ProductChange, error) {
	return s.changeRepo.List(ctx, search)
}

func (s *ProductChangeService) RecordFieldChange(ctx context.Context, productID int64, field string, oldVal, newVal *string) error {
	change := model.ProductChange{
		ProductID: productID,
		Field:     field,
		OldValue:  oldVal,
		NewValue:  newVal,
	}
	return s.changeRepo.Save(ctx, change)
}
