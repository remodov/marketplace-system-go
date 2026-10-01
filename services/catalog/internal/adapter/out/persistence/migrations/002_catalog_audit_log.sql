-- +goose Up
CREATE TABLE catalog_audit_log (
    id          uuid PRIMARY KEY,
    actor_id    uuid        NOT NULL,
    action      varchar(64) NOT NULL,
    product_id  uuid        NOT NULL,
    occurred_at timestamptz NOT NULL DEFAULT now(),
    metadata    jsonb       NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_catalog_audit_log_product_id ON catalog_audit_log (product_id);
CREATE INDEX idx_catalog_audit_log_actor_id ON catalog_audit_log (actor_id);

-- +goose Down
DROP TABLE catalog_audit_log;
