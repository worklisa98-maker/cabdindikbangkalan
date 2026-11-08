-- +goose Up
ALTER TABLE student_higher_ed_applications
	DROP CONSTRAINT IF EXISTS student_higher_ed_applications_pilihan_ke_check;

ALTER TABLE student_higher_ed_applications
	ADD CONSTRAINT student_higher_ed_applications_pilihan_ke_check
	CHECK (pilihan_ke BETWEEN 1 AND 4);

ALTER TABLE outcome_pt_offers
	DROP CONSTRAINT IF EXISTS outcome_pt_offers_pilihan_ke_check;

ALTER TABLE outcome_pt_offers
	ADD CONSTRAINT outcome_pt_offers_pilihan_ke_check
	CHECK (pilihan_ke IS NULL OR pilihan_ke BETWEEN 1 AND 4);

CREATE TABLE IF NOT EXISTS pt_choice_rules (
	id BIGSERIAL PRIMARY KEY,
	tahun_lulus INTEGER NULL,
	jalur TEXT NOT NULL,
	max_choices SMALLINT NOT NULL CHECK (max_choices BETWEEN 1 AND 4),
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	CONSTRAINT pt_choice_rules_jalur_check CHECK (jalur IN ('SNBP', 'SNBT', 'MANDIRI', 'KEDINASAN', 'LAINNYA'))
);

CREATE UNIQUE INDEX IF NOT EXISTS pt_choice_rules_unique_active_default
	ON pt_choice_rules (jalur)
	WHERE tahun_lulus IS NULL AND is_active = TRUE;

CREATE UNIQUE INDEX IF NOT EXISTS pt_choice_rules_unique_active_year
	ON pt_choice_rules (tahun_lulus, jalur)
	WHERE tahun_lulus IS NOT NULL AND is_active = TRUE;

INSERT INTO pt_choice_rules (tahun_lulus, jalur, max_choices, is_active)
VALUES
	(NULL, 'SNBP', 4, TRUE),
	(NULL, 'SNBT', 4, TRUE),
	(NULL, 'MANDIRI', 4, TRUE),
	(NULL, 'KEDINASAN', 4, TRUE),
	(NULL, 'LAINNYA', 4, TRUE)
ON CONFLICT DO NOTHING;

-- +goose Down
DROP INDEX IF EXISTS pt_choice_rules_unique_active_year;
DROP INDEX IF EXISTS pt_choice_rules_unique_active_default;
DROP TABLE IF EXISTS pt_choice_rules;

ALTER TABLE outcome_pt_offers
	DROP CONSTRAINT IF EXISTS outcome_pt_offers_pilihan_ke_check;

ALTER TABLE outcome_pt_offers
	ADD CONSTRAINT outcome_pt_offers_pilihan_ke_check
	CHECK (pilihan_ke IS NULL OR pilihan_ke BETWEEN 1 AND 3);

ALTER TABLE student_higher_ed_applications
	DROP CONSTRAINT IF EXISTS student_higher_ed_applications_pilihan_ke_check;

ALTER TABLE student_higher_ed_applications
	ADD CONSTRAINT student_higher_ed_applications_pilihan_ke_check
	CHECK (pilihan_ke BETWEEN 1 AND 3);
