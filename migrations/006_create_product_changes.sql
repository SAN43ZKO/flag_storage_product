CREATE TABLE IF NOT EXISTS product_changes (
    id          BIGSERIAL PRIMARY KEY,
    product_id  INT NOT NULL,
    field       TEXT NOT NULL,
    old_value   TEXT,
    new_value   TEXT,
    changed_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_product_changes_product_id ON product_changes(product_id);
