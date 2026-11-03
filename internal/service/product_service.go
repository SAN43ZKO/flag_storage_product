package service

import (
	"context"
	"errors"
	"log"
	"storage/internal/model"
	"storage/internal/repository"
	"strconv"
)

var (
	ErrNameRequired     = errors.New("name is required")
	ErrSKURequired      = errors.New("sku is required")
	ErrQuantityNegative = errors.New("quantity cannot be negative")
	ErrProductNotFound  = errors.New("product not found")
)

type ProductService struct {
	repo      *repository.ProductRepo
	changeSvc *ProductChangeService
}

func NewProductService(repo *repository.ProductRepo, changeSvc *ProductChangeService) *ProductService {
	return &ProductService{repo: repo, changeSvc: changeSvc}
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
	product, err := s.repo.Create(ctx, req)
	if err != nil {
		return model.Product{}, err
	}
	if s.changeSvc != nil {
		if err := s.changeSvc.RecordFieldChange(ctx, product.ID, "created", nil, strPtr("created")); err != nil {
			log.Printf("Create: failed to record change for product %d: %v", product.ID, err)
		}
	}
	return product, nil
}

func (s *ProductService) GetByID(ctx context.Context, id int64) (model.Product, error) {
	p, err := s.repo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return model.Product{}, ErrProductNotFound
	}
	return p, err
}

func (s *ProductService) List(ctx context.Context, search string, category string, sortByQty string) ([]model.Product, error) {
	return s.repo.List(ctx, search, category, sortByQty)
}

func (s *ProductService) Update(ctx context.Context, id int64, req model.CreateProductRequest) (model.Product, error) {
	if err := s.Validate(req); err != nil {
		return model.Product{}, err
	}
	oldProduct, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.Product{}, ErrProductNotFound
		}
		return model.Product{}, err
	}
	updatedProduct, err := s.repo.Update(ctx, id, req)
	if err != nil {
		return model.Product{}, err
	}

	s.recoredChange(ctx, oldProduct, updatedProduct)
	return updatedProduct, nil
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
		ImagePath: &filename,
		Reserved:  product.Reserved,
	}
	log.Printf("Calling Update with ImagePath=%q", filename)
	_, err = s.repo.Update(ctx, id, req)
	return err
}

func (s *ProductService) Delete(ctx context.Context, id int64) error {
	oldProduct, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	var name string
	if oldProduct.Name != nil {
		name = *oldProduct.Name
	}
	if err := s.changeSvc.RecordFieldChange(ctx, id, "deleted", &name, nil); err != nil {
		log.Printf("Delete: failed to record change for product %d: %v", id, err)
	}
	return nil
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
	return s.repo.DistinctCategories(ctx)
	// products, err := s.repo.List(ctx, "", "", "")
	// if err != nil {
	// 	return nil, err
	// }
	// cats := make(map[string]struct{})
	// for _, p := range products {
	// 	if p.Category != nil && *p.Category != "" {
	// 		cats[*p.Category] = struct{}{}
	// 	}
	// }
	// var res []string
	// for c := range cats {
	// 	res = append(res, c)
	// }
	// return res, nil
}

func (s *ProductService) GetLowStock(ctx context.Context, threshold int) ([]model.Product, error) {
	return s.repo.LowStock(ctx, threshold)
}

func (s *ProductService) GetUnits(ctx context.Context) ([]string, error) {
	return s.repo.DistinctUnits(ctx)
}

func (s *ProductService) FindBySKU(ctx context.Context, sku string) (model.Product, error) {
	return s.repo.FindBySKU(ctx, sku)
}

func (s *ProductService) FindByName(ctx context.Context, name string) (model.Product, error) {
	return s.repo.FindByName(ctx, name)
}

func (s *ProductService) Autocomplete(ctx context.Context, query string) ([]model.ProductAutocomplete, error) {
	return s.repo.Autocomplete(ctx, query)
}

func (s *ProductService) recoredChange(ctx context.Context, oldProd, newProd model.Product) {
	equalPtr := func(a, b *string) bool {
		if a == nil && b == nil {
			return true
		}
		if a == nil {
			return *b == ""
		}
		if b == nil {
			return *a == ""
		}
		return *a == *b
	}
	compare := func(field string, oldVal, newVal *string) {
		if equalPtr(oldVal, newVal) {
			return
		}
		if err := s.changeSvc.RecordFieldChange(ctx, newProd.ID, field, oldVal, newVal); err != nil {
			log.Printf("recoredChange: failed to record change for product %d field %s: %v", newProd.ID, field, err)
		}
	}

	compare("name", oldProd.Name, newProd.Name)
	compare("sku", oldProd.SKU, newProd.SKU)
	compare("category", oldProd.Category, newProd.Category)
	compare("unit", oldProd.Unit, newProd.Unit)
	compare("image_path", oldProd.ImagePath, newProd.ImagePath)

	oldQty := intToStrPtr(oldProd.Quantity)
	newQty := intToStrPtr(newProd.Quantity)
	compare("quantity", oldQty, newQty)

	oldRes := intToStrPtr(oldProd.Reserved)
	newRes := intToStrPtr(newProd.Reserved)
	compare("reserved", oldRes, newRes)
}

func strPtr(s string) *string   { return &s }
func intToStrPtr(i int) *string { s := strconv.Itoa(i); return &s }
