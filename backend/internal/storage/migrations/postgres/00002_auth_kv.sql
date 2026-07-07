-- +goose Up
CREATE TABLE auth_kv (
    namespace text NOT NULL,
    record_key text NOT NULL,
    owner_key text NOT NULL DEFAULT '',
    secondary_key text NOT NULL DEFAULT '',
    data jsonb NOT NULL,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (namespace, record_key)
);
CREATE UNIQUE INDEX auth_kv_secondary_unique
    ON auth_kv(namespace, secondary_key) WHERE secondary_key <> '';
CREATE INDEX auth_kv_owner_idx ON auth_kv(namespace, owner_key);
CREATE INDEX auth_kv_expires_idx ON auth_kv(namespace, expires_at);

-- +goose Down
DROP TABLE IF EXISTS auth_kv;
