-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS student_final_request_approvers (
  user_id INT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  created_by INT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS student_final_requests (
  id BIGSERIAL PRIMARY KEY,
  student_id BIGINT NOT NULL REFERENCES class12_students(id) ON DELETE RESTRICT,
  school_id INT NOT NULL REFERENCES data_sekolah(id) ON DELETE RESTRICT,
  request_type TEXT NOT NULL,
  reason TEXT NOT NULL,
  school_note TEXT NOT NULL,
  support_link TEXT NULL,
  status TEXT NOT NULL DEFAULT 'PENDING',
  submitted_by INT NULL REFERENCES users(id) ON DELETE SET NULL,
  submitted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  processed_by INT NULL REFERENCES users(id) ON DELETE SET NULL,
  processed_at TIMESTAMPTZ NULL,
  processor_note TEXT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (request_type IN ('NONAKTIF', 'KOREKSI')),
  CHECK (status IN ('PENDING', 'APPROVED', 'REJECTED'))
);

CREATE INDEX IF NOT EXISTS student_final_requests_student_idx
  ON student_final_requests (student_id, request_type, status);

CREATE INDEX IF NOT EXISTS student_final_requests_school_status_idx
  ON student_final_requests (school_id, request_type, status, submitted_at DESC);

CREATE UNIQUE INDEX IF NOT EXISTS student_final_requests_one_pending_per_type_idx
  ON student_final_requests (student_id, request_type)
  WHERE status = 'PENDING';

CREATE TABLE IF NOT EXISTS cabdin_service_info_delegates (
  user_id INT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  created_by INT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS cabdin_service_info_items (
  id BIGSERIAL PRIMARY KEY,
  title TEXT NOT NULL,
  category TEXT NOT NULL,
  sort_order INT NOT NULL DEFAULT 0,
  is_banner BOOLEAN NOT NULL DEFAULT FALSE,
  is_active BOOLEAN NOT NULL DEFAULT TRUE,
  is_archived BOOLEAN NOT NULL DEFAULT FALSE,
  file_storage_key TEXT NULL,
  file_name TEXT NULL,
  content_type TEXT NULL,
  file_size BIGINT NOT NULL DEFAULT 0,
  created_by INT NULL REFERENCES users(id) ON DELETE SET NULL,
  updated_by INT NULL REFERENCES users(id) ON DELETE SET NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS cabdin_service_info_items_public_idx
  ON cabdin_service_info_items (is_active, is_archived, category, sort_order, id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cabdin_service_info_items;
DROP TABLE IF EXISTS cabdin_service_info_delegates;
DROP TABLE IF EXISTS student_final_requests;
DROP TABLE IF EXISTS student_final_request_approvers;
-- +goose StatementEnd
