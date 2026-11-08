-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS initial_password_encrypted TEXT;

-- +goose Down
ALTER TABLE users
  DROP COLUMN IF EXISTS initial_password_encrypted;
