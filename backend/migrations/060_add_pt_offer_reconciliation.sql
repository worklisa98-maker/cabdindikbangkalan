-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS pt_offer_reconciliations (
  offer_id BIGINT PRIMARY KEY REFERENCES outcome_pt_offers(id) ON DELETE CASCADE,
  school_id INTEGER NOT NULL REFERENCES data_sekolah(id) ON DELETE CASCADE,
  tahun_lulus INTEGER NOT NULL,
  identity_key TEXT NOT NULL,
  reconciliation_status TEXT NOT NULL,
  reason TEXT NOT NULL,
  reconciled_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT pt_offer_reconciliations_status_check
    CHECK (reconciliation_status IN ('AUTO_FINAL', 'PENDING', 'EXISTING_FINAL', 'NOT_ACCEPTED'))
);

CREATE INDEX IF NOT EXISTS pt_offer_reconciliations_scope_idx
  ON pt_offer_reconciliations (school_id, tahun_lulus, identity_key);

CREATE INDEX IF NOT EXISTS pt_offer_reconciliations_auto_final_idx
  ON pt_offer_reconciliations (school_id, tahun_lulus)
  WHERE reconciliation_status = 'AUTO_FINAL';

CREATE INDEX IF NOT EXISTS outcome_pt_offers_path_status_idx
  ON outcome_pt_offers (school_id, tahun_lulus, normalized_student_name, jalur, announcement_status)
  WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- Reconciliation metadata is intentionally retained on rollback so an operator
-- can inspect decisions made while this migration was active.
