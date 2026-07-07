-- +goose Up
CREATE TABLE auth_kv (
    namespace TEXT NOT NULL,
    record_key TEXT NOT NULL,
    owner_key TEXT NOT NULL DEFAULT '',
    secondary_key TEXT NOT NULL DEFAULT '',
    data TEXT NOT NULL CHECK (json_valid(data)),
    expires_at TEXT,
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
    PRIMARY KEY (namespace, record_key)
);
CREATE UNIQUE INDEX auth_kv_secondary_unique
    ON auth_kv(namespace, secondary_key) WHERE secondary_key <> '';
CREATE INDEX auth_kv_owner_idx ON auth_kv(namespace, owner_key);
CREATE INDEX auth_kv_expires_idx ON auth_kv(namespace, expires_at);

-- +goose Down
DROP TABLE IF EXISTS auth_kv;
