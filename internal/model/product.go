package model

import "time"

type Product struct {
	ID        int64     `json:"id"`
	Name      *string   `json:"name"`
	SKU       *string   `json:"sku"`
	Quantity  int       `json:"quantity"`
	Unit      *string   `json:"unit,omitempty"`
	Category  *string   `json:"category,omitempty"`
	ImagePath *string   `json:"image_path,omitempty"`
	Reserved  int       `json:"reserved"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateProductRequest struct {
	Name      *string `json:"name"`
	SKU       *string `json:"sku"`
	Quantity  int     `json:"quantity"`
	Unit      *string `json:"unit,omitempty"`
	Category  *string `json:"category,omitempty"`
	ImagePath *string `json:"image_path,omitempty"`
	Reserved  int     `json:"reserved"`
}

type ProductAutocomplete struct {
	ID       int64  `json:"id"`
	SKU      string `json:"sku"`
	Name     string `json:"name"`
	Category string `json:"category,omitempty"`
	Unit     string `json:"unit,omitempty"`
}
