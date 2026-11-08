-- +goose Up
ALTER TABLE competition_poster_entries
  ALTER COLUMN created_by DROP NOT NULL;

ALTER TABLE competition_poster_entries
  ADD COLUMN IF NOT EXISTS submitter_type TEXT NOT NULL DEFAULT 'SCHOOL_ADMIN';

-- +goose StatementBegin
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'competition_poster_entries_submitter_type_check'
  ) THEN
    ALTER TABLE competition_poster_entries
      ADD CONSTRAINT competition_poster_entries_submitter_type_check
      CHECK (submitter_type IN ('SCHOOL_ADMIN', 'PUBLIC'));
  END IF;
END $$;
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS competition_poster_entries_submitter_type_idx
  ON competition_poster_entries (submitter_type);

-- +goose Down
DROP INDEX IF EXISTS competition_poster_entries_submitter_type_idx;

ALTER TABLE competition_poster_entries
  DROP CONSTRAINT IF EXISTS competition_poster_entries_submitter_type_check;

DELETE FROM competition_poster_entries
WHERE submitter_type = 'PUBLIC'
   OR created_by IS NULL;

ALTER TABLE competition_poster_entries
  DROP COLUMN IF EXISTS submitter_type;

ALTER TABLE competition_poster_entries
  ALTER COLUMN created_by SET NOT NULL;