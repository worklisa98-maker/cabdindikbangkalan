-- +goose Up
ALTER TABLE IF EXISTS student_final_outcomes
  DROP CONSTRAINT IF EXISTS student_final_outcomes_final_type_check;

ALTER TABLE IF EXISTS student_final_outcomes
  ADD CONSTRAINT student_final_outcomes_final_type_check
  CHECK (final_type IN ('HIGHER_ED', 'WORK', 'ENTREPRENEUR', 'UNDEFINED'));

CREATE TABLE IF NOT EXISTS legacy_import_settings (
  id SMALLINT PRIMARY KEY,
  global_enabled BOOLEAN NOT NULL DEFAULT TRUE,
  auto_disable_year INTEGER NOT NULL DEFAULT 2027,
  force_enabled BOOLEAN NOT NULL DEFAULT FALSE,
  max_year INTEGER NOT NULL DEFAULT 2025,
  updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

INSERT INTO legacy_import_settings (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS legacy_import_level_settings (
  id BIGSERIAL PRIMARY KEY,
  scope_type TEXT NOT NULL CHECK (scope_type IN ('MASTER', 'CABDIN')),
  scope_id BIGINT,
  level TEXT NOT NULL CHECK (level IN ('SMA', 'SMK', 'SLB')),
  enabled BOOLEAN NOT NULL,
  updated_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  UNIQUE (scope_type, scope_id, level),
  CHECK (
    (scope_type = 'MASTER' AND scope_id IS NULL) OR
    (scope_type = 'CABDIN' AND scope_id IS NOT NULL)
  )
);

-- +goose Down
DROP TABLE IF EXISTS legacy_import_level_settings;
DROP TABLE IF EXISTS legacy_import_settings;

ALTER TABLE IF EXISTS student_final_outcomes
  DROP CONSTRAINT IF EXISTS student_final_outcomes_final_type_check;

ALTER TABLE IF EXISTS student_final_outcomes
  ADD CONSTRAINT student_final_outcomes_final_type_check
  CHECK (final_type IN ('HIGHER_ED'));
