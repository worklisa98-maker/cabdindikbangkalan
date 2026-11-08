package services

import (
	"strings"
	"testing"
)

func TestBuildUpsertUserRoleStatementWithSchoolID(t *testing.T) {
	schoolID := 77
	query, args := buildUpsertUserRoleStatement(2026, 10, GaspolRoleSchoolEntry, &schoolID, 99)

	if len(args) != 5 {
		t.Fatalf("expected 5 args for school-scoped role, got %d", len(args))
	}
	if !strings.Contains(query, "VALUES ($1::integer, $2::integer, $3::gaspol_role_type, $4::integer, TRUE, $5::integer)") {
		t.Fatalf("expected explicit casts in school-scoped upsert query, got %s", query)
	}
}

func TestBuildUpsertUserRoleStatementWithoutSchoolID(t *testing.T) {
	query, args := buildUpsertUserRoleStatement(2026, 10, GaspolRoleSupervisor, nil, 99)

	if len(args) != 4 {
		t.Fatalf("expected 4 args for non-school role, got %d", len(args))
	}
	if !strings.Contains(query, "VALUES ($1::integer, $2::integer, $3::gaspol_role_type, NULL, TRUE, $4::integer)") {
		t.Fatalf("expected non-school upsert query to skip school placeholder, got %s", query)
	}
	if strings.Contains(query, "$5::integer") {
		t.Fatalf("non-school upsert query must not reference a fifth parameter: %s", query)
	}
}

func TestBuildCreateGaspolSchoolUserStatementWithInitialPasswordV2(t *testing.T) {
	schoolID := 77
	ciphertext := "ciphertext"
	nonce := "nonce"

	query, args := buildCreateGaspolSchoolUserStatement(
		"gaspol-12345678",
		"hashed-password",
		"SCHOOL_ADMIN",
		&schoolID,
		"GASPOL SMAN 1",
		gaspolInitialPasswordRecord{
			Ciphertext: &ciphertext,
			Nonce:      &nonce,
		},
	)

	if len(args) != 7 {
		t.Fatalf("expected 7 args when storing v2 initial password, got %d", len(args))
	}
	if !strings.Contains(query, "initial_password_ciphertext") || !strings.Contains(query, "initial_password_nonce") {
		t.Fatalf("expected initial password v2 columns in query, got %s", query)
	}
	if !strings.Contains(query, "initial_password_set_at") {
		t.Fatalf("expected initial_password_set_at to be written, got %s", query)
	}
}

func TestBuildCreateGaspolSchoolUserStatementWithoutInitialPassword(t *testing.T) {
	schoolID := 77
	query, args := buildCreateGaspolSchoolUserStatement(
		"gaspol-12345678",
		"hashed-password",
		"SCHOOL_ADMIN",
		&schoolID,
		"GASPOL SMAN 1",
		gaspolInitialPasswordRecord{},
	)

	if len(args) != 5 {
		t.Fatalf("expected 5 args without initial password columns, got %d", len(args))
	}
	if strings.Contains(query, "initial_password_") {
		t.Fatalf("query must not reference initial password columns when not provided: %s", query)
	}
}

func TestUpsertSubmissionQueryUsesExplicitStatusCast(t *testing.T) {
	query := `
		INSERT INTO gaspol_school_submissions (
			year,
			month,
			school_id,
			submitted_by,
			report_title,
			report_body,
			report_link,
			attachment_urls,
			status,
			submitted_at
		)
		VALUES (
			$1,
			$2,
			$3,
			$4,
			$5,
			$6,
			$7,
			$8::jsonb,
			$9::gaspol_submission_status_type,
			CASE
				WHEN $9::gaspol_submission_status_type = 'SUBMITTED'::gaspol_submission_status_type THEN NOW()
				ELSE NULL
			END
		)
	`

	if !strings.Contains(query, "$9::gaspol_submission_status_type") {
		t.Fatalf("expected status parameter to use explicit enum cast, got %s", query)
	}
	if !strings.Contains(query, "WHEN $9::gaspol_submission_status_type = 'SUBMITTED'::gaspol_submission_status_type") {
		t.Fatalf("expected CASE expression to use the same explicit cast, got %s", query)
	}
}
