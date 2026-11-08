-- +goose Up
ALTER TYPE role_type ADD VALUE IF NOT EXISTS 'GASPOL_VOTER';
-- +goose Down
SELECT 1; -- Enum retained for compatibility with existing accounts.
