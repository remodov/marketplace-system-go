-- +goose Up
CREATE TABLE outbox (
    id             uuid PRIMARY KEY,
    aggregate_id   uuid NOT NULL,
    aggregate_type varchar(64) NOT NULL,
    event_type     varchar(128) NOT NULL,
    event_version  int NOT NULL DEFAULT 1,
    payload        jsonb NOT NULL,
    occurred_at    timestamptz NOT NULL,
    published_at   timestamptz
);

CREATE INDEX idx_outbox_unpublished ON outbox (occurred_at) WHERE published_at IS NULL;
CREATE INDEX idx_outbox_aggregate ON outbox (aggregate_id, occurred_at);

-- +goose Down
DROP TABLE outbox;
