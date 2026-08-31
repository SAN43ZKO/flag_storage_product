package repository

import (
	"context"
	"fmt"

	"storage/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ProductChangeRepo struct {
	pool *pgxpool.Pool
}

func NewProductChangeRepo(pool *pgxpool.Pool) *ProductChangeRepo {
	return &ProductChangeRepo{pool: pool}
}

func (r *ProductChangeRepo) Save(ctx context.Context, change model.ProductChange) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO product_changes (product_id, field, old_value, new_value)
         VALUES ($1, $2, $3, $4)`,
		change.ProductID, change.Field, change.OldValue, change.NewValue)
	return err
}

func (r *ProductChangeRepo) List(ctx context.Context, search string) ([]model.ProductChange, error) {
	query := `SELECT id, product_id, field, old_value, new_value, changed_at
              FROM product_changes`
	args := []interface{}{}
	if search != "" {
		query += ` WHERE product_id IN (
            SELECT id FROM products WHERE name ILIKE $1 OR sku ILIKE $2
        )`
		like := "%" + search + "%"
		args = append(args, like, like)
	}
	query += ` ORDER BY changed_at DESC LIMIT 500`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list product changes: %w", err)
	}
	defer rows.Close()

	var changes []model.ProductChange
	for rows.Next() {
		var c model.ProductChange
		if err := rows.Scan(&c.ID, &c.ProductID, &c.Field, &c.OldValue, &c.NewValue, &c.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan product change: %w", err)
		}
		changes = append(changes, c)
	}
	return changes, rows.Err()
}
