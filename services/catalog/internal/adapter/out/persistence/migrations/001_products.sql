-- +goose Up
CREATE TYPE product_status AS ENUM ('DRAFT', 'PUBLISHED', 'HIDDEN');

CREATE TABLE products (
    id          uuid PRIMARY KEY,
    title       varchar(255)   NOT NULL,
    description text,
    price       numeric(15, 2) NOT NULL,
    currency    varchar(3)     NOT NULL DEFAULT 'RUB',
    seller_id   uuid           NOT NULL,
    status      product_status NOT NULL,
    created_at  timestamptz    NOT NULL DEFAULT now(),
    updated_at  timestamptz    NOT NULL DEFAULT now(),
    CONSTRAINT products_price_positive CHECK (price > 0)
);

CREATE INDEX idx_products_seller_id ON products (seller_id);
CREATE INDEX idx_products_status ON products (status);

-- +goose Down
DROP TABLE products;
DROP TYPE product_status;
