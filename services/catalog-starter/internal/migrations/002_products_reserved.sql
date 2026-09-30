-- +goose Up
ALTER TABLE products ADD COLUMN reserved int NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE products DROP COLUMN reserved;
