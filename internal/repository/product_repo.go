package repository

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"storage/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound — sentinel error for "record not found" cases.
var ErrNotFound = errors.New("not found")

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
		reserved,
		created_at,
		updated_at)
	VALUES($1, $2, $3, $4, $5, $6, $7, $8, $9)
	RETURNING id, name, sku, quantity, unit, category, image_path, reserved, created_at, updated_at`
	err := r.pool.QueryRow(ctx, createQuery, req.Name, req.SKU, req.Quantity, req.Unit, req.Category, req.ImagePath, req.Reserved, now, now).Scan(
		&p.ID,
		&p.Name,
		&p.SKU,
		&p.Quantity,
		&p.Unit,
		&p.Category,
		&p.ImagePath,
		&p.Reserved,
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
	reserved,
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
		&p.Reserved,
		&p.CreatedAt,
		&p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.Product{}, fmt.Errorf("GetByID(1): %w", ErrNotFound)
		}
		return model.Product{}, fmt.Errorf("GetByID(2): get product: %w", err)
	}
	return p, nil
}

func (r *ProductRepo) List(ctx context.Context, search string, category string, sortByQty string) ([]model.Product, error) {
	query := `SELECT id, name, sku, quantity, category, unit, image_path, reserved, created_at, updated_at FROM products`
	args := []interface{}{}
	conditions := []string{}

	if search != "" {
		like := "%" + search + "%"
		args = append(args, like, like)
		conditions = append(conditions, "(name ILIKE $1 OR sku ILIKE $2)")
	}
	if category != "" {
		args = append(args, category)
		conditions = append(conditions, fmt.Sprintf("category = $%d", len(args)))
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if sortByQty == "asc" {
		query += " ORDER BY quantity ASC"
	} else if sortByQty == "desc" {
		query += " ORDER BY quantity DESC"
	} else {
		query += " ORDER BY id"
	}

	rows, err := r.pool.Query(ctx, query, args...)
	// var (
	// 	rows pgx.Rows
	// 	err  error
	// )

	// if search == "" {
	// 	rows, err = r.pool.Query(ctx,
	// 		`SELECT id, name, sku, quantity, unit, category, image_path, reserved, created_at, updated_at
	// 	FROM products ORDER BY id`)
	// } else {
	// 	like := "%" + search + "%"
	// 	rows, err = r.pool.Query(ctx,
	// 		`SELECT id, name, sku, quantity, unit, category, image_path, reserved, created_at, updated_at
	// 	FROM products
	// 	WHERE name ILIKE $1 OR sku ILIKE $2
	// 	ORDER BY id`, like, like)
	// }

	if err != nil {
		return nil, fmt.Errorf("list pruducts: %w", err)
	}
	defer rows.Close()

	var products []model.Product
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.SKU, &p.Quantity, &p.Category, &p.Unit, &p.ImagePath, &p.Reserved, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("List(2): scan product: %w", err)
		}
		products = append(products, p)
	}
	return products, rows.Err()
}

// Updates all fields of the product without id and created_at
func (r *ProductRepo) Update(ctx context.Context, id int64, req model.CreateProductRequest) (model.Product, error) {
	log.Printf("Update repo called: id=%d, ImagePath=%v, reserved=%v", id, req.ImagePath, req.Reserved)
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
		reserved=$7,
		updated_at=$8
	WHERE id=$9
	RETURNING id, name, sku, quantity, unit, category, image_path, reserved, created_at, updated_at`
	err := r.pool.QueryRow(ctx, updateQuery, req.Name, req.SKU, req.Quantity, req.Unit, req.Category, req.ImagePath, req.Reserved, now, id).Scan(
		&p.ID,
		&p.Name,
		&p.SKU,
		&p.Quantity,
		&p.Unit,
		&p.Category,
		&p.ImagePath,
		&p.Reserved,
		&p.CreatedAt,
		&p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.Product{}, fmt.Errorf("Update(1): %w", ErrNotFound)
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

func (r *ProductRepo) Count(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM products`).Scan(&count)
	return count, err
}

func (r *ProductRepo) CountCategories(ctx context.Context) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(DISTINCT category) FROM products WHERE category IS NOT NULL AND category <> ''`).Scan(&count)
	return count, err
}

func (r *ProductRepo) LowStock(ctx context.Context, threshold int) ([]model.Product, error) {
	rows, err := r.pool.Query(ctx, `SELECT id, name, sku, quantity, category, unit, image_path, reserved, created_at, updated_at FROM products WHERE quantity < $1 ORDER BY quantity`, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var products []model.Product
	for rows.Next() {
		var p model.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.SKU, &p.Quantity, &p.Category, &p.Unit, &p.ImagePath, &p.Reserved, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *ProductRepo) DistinctCategories(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT DISTINCT category FROM products
         WHERE category IS NOT NULL AND category <> ''
         ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var categories []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (r *ProductRepo) DistinctUnits(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT unit FROM products WHERE unit IS NOT NULL AND unit <> '' ORDER BY unit`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var units []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		units = append(units, u)
	}
	return units, rows.Err()
}

func (r *ProductRepo) Autocomplete(ctx context.Context, query string) ([]model.ProductAutocomplete, error) {
	like := "%" + query + "%"
	rows, err := r.pool.Query(ctx,
		`SELECT id, COALESCE(sku, ''), COALESCE(name, ''), COALESCE(category, ''), COALESCE(unit, '')
         FROM products
         WHERE name ILIKE $1 OR sku ILIKE $2
         ORDER BY name
         LIMIT 10`, like, like)
	if err != nil {
		return nil, fmt.Errorf("autocomplete: %w", err)
	}
	defer rows.Close()

	var items []model.ProductAutocomplete
	for rows.Next() {
		var item model.ProductAutocomplete
		if err := rows.Scan(&item.ID, &item.SKU, &item.Name, &item.Category, &item.Unit); err != nil {
			return nil, fmt.Errorf("scan autocomplete: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *ProductRepo) FindBySKU(ctx context.Context, sku string) (model.Product, error) {
	var p model.Product
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, sku, quantity, category, unit, image_path, reserved, created_at, updated_at
         FROM products WHERE sku = $1`, sku,
	).Scan(&p.ID, &p.Name, &p.SKU, &p.Quantity, &p.Category, &p.Unit, &p.ImagePath, &p.Reserved, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.Product{}, ErrNotFound
		}
		return model.Product{}, fmt.Errorf("find by sku: %w", err)
	}
	return p, nil
}

func (r *ProductRepo) FindByName(ctx context.Context, name string) (model.Product, error) {
	var p model.Product
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, sku, quantity, category, unit, image_path, reserved, created_at, updated_at
         FROM products WHERE name = $1`, name,
	).Scan(&p.ID, &p.Name, &p.SKU, &p.Quantity, &p.Category, &p.Unit, &p.ImagePath, &p.Reserved, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return model.Product{}, ErrNotFound
		}
		return model.Product{}, fmt.Errorf("find by name: %w", err)
	}
	return p, nil
}
