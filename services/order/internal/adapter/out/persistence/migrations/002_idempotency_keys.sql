-- +goose Up
CREATE TABLE idempotency_keys (
    idempotency_key varchar(128) PRIMARY KEY,
    request_hash    varchar(64) NOT NULL,
    order_id        uuid NOT NULL REFERENCES orders (id),
    created_at      timestamptz NOT NULL
);

CREATE INDEX idx_idempotency_keys_created ON idempotency_keys (created_at);

-- +goose Down
DROP TABLE idempotency_keys;
