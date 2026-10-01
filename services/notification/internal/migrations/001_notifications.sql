-- +goose Up
CREATE TABLE processed_events (
    event_id     uuid PRIMARY KEY,
    event_type   varchar(128) NOT NULL,
    processed_at timestamptz NOT NULL
);

CREATE INDEX idx_processed_events_processed_at ON processed_events (processed_at);

CREATE TABLE notifications (
    id           uuid PRIMARY KEY,
    event_id     uuid NOT NULL,
    event_type   varchar(64) NOT NULL,
    user_id      uuid NOT NULL,
    channel      varchar(16) NOT NULL,
    template_key varchar(128) NOT NULL,
    status       varchar(16) NOT NULL,
    payload      jsonb NOT NULL,
    created_at   timestamptz NOT NULL
);

CREATE INDEX idx_notifications_user_created ON notifications (user_id, created_at DESC);
CREATE INDEX idx_notifications_status_created ON notifications (status, created_at);

-- +goose Down
DROP TABLE notifications;
DROP TABLE processed_events;
