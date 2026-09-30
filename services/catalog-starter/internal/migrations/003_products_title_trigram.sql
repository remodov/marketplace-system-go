-- +goose Up
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_products_title_trgm ON products USING gin (title gin_trgm_ops);

-- +goose Down
DROP INDEX idx_products_title_trgm;
