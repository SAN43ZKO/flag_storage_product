package model

import "time"

type ProductChange struct {
	ID        int64     `json:"id"`
	ProductID int64     `json:"product_id"`
	Field     string    `json:"field"`
	OldValue  *string   `json:"old_value,omitempty"`
	NewValue  *string   `json:"new_value,omitempty"`
	ChangedAt time.Time `json:"changed_at"`
}
