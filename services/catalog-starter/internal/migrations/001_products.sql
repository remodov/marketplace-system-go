-- +goose Up
CREATE TABLE products (
    id      uuid PRIMARY KEY,
    title   varchar(255)  NOT NULL,
    price   numeric(12,2) NOT NULL,
    stock   int           NOT NULL,
    version bigint        NOT NULL DEFAULT 0
);
CREATE INDEX idx_products_title ON products (title);

-- +goose Down
DROP TABLE products;
