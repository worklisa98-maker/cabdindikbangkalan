-- +goose Up
-- +goose StatementBegin
ALTER TABLE guide_books
  ADD COLUMN IF NOT EXISTS placement_key TEXT,
  ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS guide_book_slides (
  id BIGSERIAL PRIMARY KEY,
  guide_book_id BIGINT NOT NULL REFERENCES guide_books(id) ON DELETE CASCADE,
  layout_type TEXT NOT NULL CHECK (
    layout_type IN (
      'TEXT_LEFT_IMAGE_RIGHT',
      'IMAGE_LEFT_TEXT_RIGHT',
      'IMAGE_LEFT_IMAGE_RIGHT',
      'FULL_TEXT',
      'FULL_IMAGE',
      'TEXT_TOP_TWO_IMAGES'
    )
  ),
  text_html TEXT NOT NULL DEFAULT '',
  image_1_url TEXT NOT NULL DEFAULT '',
  image_2_url TEXT NOT NULL DEFAULT '',
  column_ratio TEXT NOT NULL DEFAULT '1:1',
  sort_order INT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS guide_book_slides_guide_book_id_idx
  ON guide_book_slides (guide_book_id, sort_order, id);

CREATE UNIQUE INDEX IF NOT EXISTS guide_books_active_placement_unique_idx
  ON guide_books (placement_key)
  WHERE is_active = TRUE
    AND placement_key IS NOT NULL
    AND btrim(placement_key) <> '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS guide_books_active_placement_unique_idx;
DROP INDEX IF EXISTS guide_book_slides_guide_book_id_idx;
DROP TABLE IF EXISTS guide_book_slides;
ALTER TABLE guide_books
  DROP COLUMN IF EXISTS is_public,
  DROP COLUMN IF EXISTS placement_key;
-- +goose StatementEnd
