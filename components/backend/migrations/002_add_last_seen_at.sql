-- +goose Up
ALTER TABLE users
ADD COLUMN last_seen_at TIMESTAMP;

CREATE INDEX IF NOT EXISTS users_last_seen_idx ON users (last_seen_at);

-- +goose Down
DROP INDEX IF EXISTS users_last_seen_idx;
ALTER TABLE users
DROP COLUMN IF EXISTS last_seen_at;
