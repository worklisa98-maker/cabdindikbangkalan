-- +goose Up
ALTER TABLE users
ADD COLUMN IF NOT EXISTS allowed_levels TEXT[];

ALTER TABLE users
ALTER COLUMN allowed_levels SET DEFAULT '{}'::text[];

UPDATE users
SET allowed_levels = '{}'::text[]
WHERE allowed_levels IS NULL;

-- +goose Down
ALTER TABLE users
ALTER COLUMN allowed_levels DROP DEFAULT;
