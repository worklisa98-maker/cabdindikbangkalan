-- +goose Up
DROP INDEX IF EXISTS student_higher_ed_applications_unique_choice;

CREATE UNIQUE INDEX IF NOT EXISTS student_higher_ed_applications_unique_choice_per_jalur
  ON student_higher_ed_applications (school_id, cohort_id, jalur_penerimaan, pilihan_ke);

-- +goose Down
DROP INDEX IF EXISTS student_higher_ed_applications_unique_choice_per_jalur;

CREATE UNIQUE INDEX IF NOT EXISTS student_higher_ed_applications_unique_choice
  ON student_higher_ed_applications (school_id, cohort_id, pilihan_ke);
