package repository

import (
	"context"
	"fmt"
	"log"
	"time"

	"storage/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductRepo struct {
	pool *pgxpool.Pool
}

func NewProductRepo(pool *pgxpool.Pool) *ProductRepo {
	return &ProductRepo{pool: pool}
}

// Create new product with return id and timestamp
func (r *ProductRepo) Create(ctx context.Context, req model.CreateProductRequest) (model.Product, error) {
	var p model.Product
	now := time.Now().UTC()
	const createQuery = `
	INSERT INTO products(
		name,
		sku,
		quantity,
		unit,
		category,
		image_path,
		created_at,
		updated_at)
	VALUES($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id, name, sku, quantity, unit, category, image_path, created_at, updated_at`
	err := r.pool.QueryRow(ctx, createQuery, req.Name, req.SKU, req.Quantity, req.Unit, req.Category, req.ImagePath, now, now).Scan(
		&p.ID,
		&p.Name,
		&p.SKU,
		&p.Quantity,
		&p.Unit,
		&p.Category,
		&p.ImagePath,
		&p.CreatedAt,
		&p.UpdatedAt)
	if err != nil {
		return model.Product{}, fmt.Errorf("Create(1): insert product: %w", err)
	}
	return p, nil
}

// Get products by ID
func (r *ProductRepo) GetByID(ctx context.Context, id int64) (model.Product, error) {
	var p model.Product
	const getByIDQuery = `
	SELECT
	id,
	name,
	sku,
	quantity,
	unit,
	category,
	image_path,
	created_at,
	updated_at
	FROM products WHERE id = $1`
	err := r.pool.QueryRow(ctx, getByIDQuery, id).Scan(
		&p.ID,
		&p.Name,
		&p.SKU,
		&p.Quantity,
		&p.Unit,
		&p.Category,
		&p.ImagePath,
		&p.CreatedAt,
		&p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.Product{}, fmt.Errorf("GetByID(1): product not found")
		}
		return model.Product{}, fmt.Errorf("GetByID(2): get product: %w", err)
	}
	return p, nil
}

func (r *ProductRepo) List(ctx context.Context, search string) ([]model.Product, error) {
	var (
		rows pgx.Rows
		err  error
	)

	if search == "" {
		rows, err = r.pool.Query(ctx,
			`SELECT id, name, sku, quantity, unit, category, image_path, created_at, updated_at
		FROM products ORDER BY id`)
	} else {
		like := "%" + search + "%"
		rows, err = r.pool.Query(ctx,
			`SELECT id, name, sku, quantity, unit, category, image_path, created_at, updated_at
		FROM products
		WHERE name ILIKE $1 OR sku ILIKE $2
		ORDER BY id`, like, like)
	}

	if err != nil {
		return nil, fmt.Errorf("list pruducts: %w", err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.SKU, &p.Quantity, &p.Unit, &p.Category, &p.ImagePath, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("List(2): scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

// Updates all fields of the product without id and created_at
func (r *ProductRepo) Update(ctx context.Context, id int64, req model.CreateProductRequest) (model.Product, error) {
	log.Printf("Update repo called: id=%d, ImagePath=%q", id, req.ImagePath)
	var p model.Product
	now := time.Now().UTC()
	const updateQuery = `
	UPDATE products SET
		name=$1,
		sku=$2,
		quantity=$3,
		unit=$4,
		category=$5,
		image_path=$6,
		updated_at=$7
	WHERE id=$8
	RETURNING id, name, sku, quantity, unit, category, image_path, created_at, updated_at`
	err := r.pool.QueryRow(ctx, updateQuery, req.Name, req.SKU, req.Quantity, req.Unit, req.Category, req.ImagePath, now, id).Scan(
		&p.ID,
		&p.Name,
		&p.SKU,
		&p.Quantity,
		&p.Unit,
		&p.Category,
		&p.ImagePath,
		&p.CreatedAt,
		&p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.Product{}, fmt.Errorf("Update(1): product not found")
		}
		return model.Product{}, fmt.Errorf("Update(2): update product: %w", err)
	}
	return p, nil
}

// Delete product by id
func (r *ProductRepo) Delete(ctx context.Context, id int64) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM products WHERE id=$1`, id)
	if err != nil {
		return fmt.Errorf("Delete(1): delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("Delete(2): product not found")
	}
	return nil
}
