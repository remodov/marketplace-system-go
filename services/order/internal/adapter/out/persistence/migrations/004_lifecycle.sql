-- +goose Up
ALTER TABLE orders
    ADD COLUMN payment_id   uuid,
    ADD COLUMN paid_at      timestamptz,
    ADD COLUMN shipped_at   timestamptz,
    ADD COLUMN delivered_at timestamptz,
    ADD COLUMN closed_at    timestamptz;

CREATE INDEX idx_orders_pending_payment_created ON orders (created_at) WHERE status = 'PENDING_PAYMENT';

CREATE TABLE processed_events (
    event_id     uuid PRIMARY KEY,
    event_type   varchar(128) NOT NULL,
    processed_at timestamptz NOT NULL
);

-- +goose Down
DROP TABLE processed_events;
DROP INDEX idx_orders_pending_payment_created;
ALTER TABLE orders
    DROP COLUMN closed_at,
    DROP COLUMN delivered_at,
    DROP COLUMN shipped_at,
    DROP COLUMN paid_at,
    DROP COLUMN payment_id;
