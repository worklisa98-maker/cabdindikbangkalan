-- +goose Up
ALTER TYPE role_type ADD VALUE IF NOT EXISTS 'USER_SEKOLAH_GASPOL';

-- +goose Down
-- PostgreSQL enum value removal is intentionally not supported.
