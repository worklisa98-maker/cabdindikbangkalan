-- +goose Up
ALTER TABLE competition_poster_entries
  ALTER COLUMN created_at DROP DEFAULT,
  ALTER COLUMN updated_at DROP DEFAULT;

ALTER TABLE competition_poster_entries
  ALTER COLUMN created_at TYPE TIMESTAMPTZ
  USING created_at AT TIME ZONE 'Asia/Jakarta',
  ALTER COLUMN updated_at TYPE TIMESTAMPTZ
  USING updated_at AT TIME ZONE 'Asia/Jakarta',
  ALTER COLUMN deleted_at TYPE TIMESTAMPTZ
  USING deleted_at AT TIME ZONE 'Asia/Jakarta';

ALTER TABLE competition_poster_entries
  ALTER COLUMN created_at SET DEFAULT NOW(),
  ALTER COLUMN updated_at SET DEFAULT NOW();

-- +goose Down
ALTER TABLE competition_poster_entries
  ALTER COLUMN created_at DROP DEFAULT,
  ALTER COLUMN updated_at DROP DEFAULT;

ALTER TABLE competition_poster_entries
  ALTER COLUMN created_at TYPE TIMESTAMP
  USING created_at AT TIME ZONE 'Asia/Jakarta',
  ALTER COLUMN updated_at TYPE TIMESTAMP
  USING updated_at AT TIME ZONE 'Asia/Jakarta',
  ALTER COLUMN deleted_at TYPE TIMESTAMP
  USING deleted_at AT TIME ZONE 'Asia/Jakarta';

ALTER TABLE competition_poster_entries
  ALTER COLUMN created_at SET DEFAULT NOW(),
  ALTER COLUMN updated_at SET DEFAULT NOW();
