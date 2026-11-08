-- +goose Up
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS initial_password_ciphertext TEXT,
  ADD COLUMN IF NOT EXISTS initial_password_nonce TEXT,
  ADD COLUMN IF NOT EXISTS initial_password_set_at TIMESTAMPTZ NULL,
  ADD COLUMN IF NOT EXISTS initial_password_version INT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE users
  DROP COLUMN IF EXISTS initial_password_ciphertext,
  DROP COLUMN IF EXISTS initial_password_nonce,
  DROP COLUMN IF EXISTS initial_password_set_at,
  DROP COLUMN IF EXISTS initial_password_version;
