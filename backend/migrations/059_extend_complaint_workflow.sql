-- +goose Up
-- +goose StatementBegin
ALTER TABLE complaints DROP CONSTRAINT IF EXISTS complaints_status_check;
ALTER TABLE complaints ADD CONSTRAINT complaints_status_check CHECK (status IN
  ('RECEIVED','VERIFICATION','FOLLOW_UP','RESPONDED','COMPLETED','NEEDS_INFO',
   'CLARIFICATION_SUBMITTED','REJECTED','REJECTED_SPAM','OUT_OF_SCOPE','REOPENED'));

ALTER TABLE complaints
  ADD COLUMN IF NOT EXISTS official_response TEXT,
  ADD COLUMN IF NOT EXISTS official_response_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS clarification_deadline TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS closed_reason TEXT;

CREATE INDEX IF NOT EXISTS complaints_clarification_deadline_idx
  ON complaints(clarification_deadline)
  WHERE status = 'NEEDS_INFO';

ALTER TABLE complaint_timeline
  ADD COLUMN IF NOT EXISTS action VARCHAR(80),
  ADD COLUMN IF NOT EXISTS actor_role VARCHAR(50),
  ADD COLUMN IF NOT EXISTS attachment_id BIGINT REFERENCES complaint_attachments(id) ON DELETE SET NULL;

UPDATE complaint_timeline
SET action = COALESCE(action, 'STATUS_CHANGED')
WHERE action IS NULL;
ALTER TABLE complaint_timeline ALTER COLUMN action SET NOT NULL;
ALTER TABLE complaint_timeline ALTER COLUMN action SET DEFAULT 'STATUS_CHANGED';

CREATE TABLE complaint_clarifications (
    id BIGSERIAL PRIMARY KEY,
    complaint_id BIGINT NOT NULL REFERENCES complaints(id) ON DELETE CASCADE,
    round_number INTEGER NOT NULL CHECK (round_number > 0),
    request_text TEXT NOT NULL CHECK (char_length(trim(request_text)) > 0),
    requested_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deadline_at TIMESTAMPTZ NOT NULL,
    response_text TEXT,
    responded_at TIMESTAMPTZ,
    review_result VARCHAR(32) CHECK (review_result IN ('ACCEPTED','MORE_INFO','REJECTED')),
    reviewed_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    review_note TEXT,
    UNIQUE (complaint_id, round_number)
);
CREATE INDEX complaint_clarifications_active_idx
  ON complaint_clarifications(complaint_id, requested_at DESC);

ALTER TABLE complaint_attachments
  ADD COLUMN IF NOT EXISTS kind VARCHAR(32) NOT NULL DEFAULT 'SUBMISSION'
    CHECK (kind IN ('SUBMISSION','FOLLOW_UP','CLARIFICATION','OFFICIAL_RESPONSE')),
  ADD COLUMN IF NOT EXISTS clarification_id BIGINT REFERENCES complaint_clarifications(id) ON DELETE SET NULL,
  ADD COLUMN IF NOT EXISTS uploaded_by INTEGER REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE complaint_audit_log
  ADD COLUMN IF NOT EXISTS actor_role VARCHAR(50),
  ADD COLUMN IF NOT EXISTS note TEXT,
  ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT FALSE,
  ADD COLUMN IF NOT EXISTS attachment_id BIGINT REFERENCES complaint_attachments(id) ON DELETE SET NULL;

UPDATE complaints SET status='REJECTED' WHERE status='REJECTED_SPAM';
-- +goose StatementEnd

-- +goose Down
-- Data-bearing workflow columns and tables are intentionally retained.
SELECT 1;
