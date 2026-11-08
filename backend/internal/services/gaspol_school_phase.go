package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"github.com/jackc/pgx/v5"
)

type GaspolSchoolNotice struct {
	ID      int64     `json:"id"`
	LockID  int64     `json:"lockId"`
	Status  string    `json:"status"`
	Message string    `json:"message"`
	Tone    string    `json:"tone"`
	EventAt time.Time `json:"eventAt"`
	Read    bool      `json:"read"`
}

func gaspolNoticeContent(status string) (string, string) {
	switch status {
	case "TOP_10":
		return "Selamat, sekolah Anda resmi masuk Top 10 GASPOL.", "success"
	case "NOT_FAILED":
		return "Sekolah Anda belum masuk Top 10. Silakan isi Laporan Nominasi Tidak Tergagal beserta bukti perbaikan.", "attention"
	case "PASSED_NOMINATION":
		return "Selamat, Anda tidak masuk Kandidat Zona Tergagal.", "success"
	case "BOTTOM_5":
		return "Sekolah Anda resmi masuk Banding Kandidat Zona Tergagal. Silakan lengkapi laporan banding dan bukti terbaru.", "candidate"
	case "ESCAPED_BOTTOM_ZONE":
		return "Selamat, berdasarkan hasil banding resmi sekolah Anda telah keluar dari Zona Tergagal.", "success"
	case "TERGAGAL":
		return "Berdasarkan hasil banding resmi, sekolah Anda ditetapkan dalam kategori Tergagal tahun ini. Hasil ini menjadi bahan evaluasi dan perbaikan bersama.", "attention"
	}
	return "", ""
}

func (s GaspolService) ensurePhaseSchool(ctx context.Context, user auth.User, year int) error {
	if year < 2024 {
		return errors.New("INVALID_YEAR")
	}
	if user.SchoolID == nil || *user.SchoolID <= 0 {
		return auth.ErrForbidden
	}
	if err := s.ensureGaspolRole(ctx, user, year, GaspolRoleSchoolEntry, user.SchoolID); err != nil {
		return err
	}
	var active bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_school_participants WHERE year=$1 AND school_id=$2 AND is_active AND NOT is_year_configuration)`, year, *user.SchoolID).Scan(&active)
	if err != nil {
		return err
	}
	if !active {
		return errors.New("SCHOOL_NOT_PARTICIPANT")
	}
	return nil
}

func (s GaspolService) SchoolNotices(ctx context.Context, user auth.User, year int) ([]GaspolSchoolNotice, error) {
	if err := s.ensurePhaseSchool(ctx, user, year); err != nil {
		return nil, err
	}
	rows, err := s.DB.Pool.Query(ctx, `SELECT c.id,l.id,c.cohort,l.locked_at,r.read_at IS NOT NULL
 FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED'
 LEFT JOIN gaspol_school_notice_reads r ON r.school_id=c.school_id AND r.year=c.year AND r.cohort_id=c.id AND r.source_lock_id=l.id
 WHERE c.year=$1 AND c.school_id=$2 AND c.released_at IS NULL AND c.cohort<>'TOP_5'
 ORDER BY l.locked_at DESC,c.id DESC`, year, *user.SchoolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []GaspolSchoolNotice{}
	for rows.Next() {
		var n GaspolSchoolNotice
		if err = rows.Scan(&n.ID, &n.LockID, &n.Status, &n.EventAt, &n.Read); err != nil {
			return nil, err
		}
		n.Message, n.Tone = gaspolNoticeContent(n.Status)
		if n.Message != "" {
			result = append(result, n)
		}
	}
	return result, rows.Err()
}

func (s GaspolService) AcknowledgeSchoolNotice(ctx context.Context, user auth.User, year int, id, lockID int64) error {
	if err := s.ensurePhaseSchool(ctx, user, year); err != nil {
		return err
	}
	tag, err := s.DB.Pool.Exec(ctx, `INSERT INTO gaspol_school_notice_reads(school_id,year,cohort_id,source_lock_id,read_by)
 SELECT c.school_id,c.year,c.id,l.id,$5 FROM gaspol_cohorts c
 JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED'
 WHERE c.school_id=$1 AND c.year=$2 AND c.id=$3 AND l.id=$4 AND c.released_at IS NULL
 AND c.cohort IN ('TOP_10','NOT_FAILED','PASSED_NOMINATION','BOTTOM_5','ESCAPED_BOTTOM_ZONE','TERGAGAL')
 ON CONFLICT(school_id,year,cohort_id,source_lock_id) DO UPDATE SET read_by=gaspol_school_notice_reads.read_by`, *user.SchoolID, year, id, lockID, user.ID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return auth.ErrForbidden
	}
	return nil
}

func normalizeGaspolEvidenceURL(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.Contains(value, "://") && !strings.Contains(value, ":") && strings.Contains(strings.Split(value, "/")[0], ".") {
		value = "https://" + value
	}
	return value
}
func validGaspolEvidenceURL(raw string) bool {
	value := normalizeGaspolEvidenceURL(raw)
	u, err := url.Parse(value)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil && !strings.ContainsAny(value, " \t\r\n")
}

func validateNominationInput(input GaspolNominationSubmissionInput) error {
	if input.Year < 2024 {
		return errors.New("INVALID_YEAR")
	}
	status := strings.ToUpper(strings.TrimSpace(input.Status))
	if status != "" && status != "DRAFT" && status != "SUBMITTED" {
		return errors.New("INVALID_STATUS")
	}
	if status == "SUBMITTED" && strings.TrimSpace(input.DriveURL) == "" {
		return errors.New("INCOMPLETE_SUBMISSION")
	}
	if strings.TrimSpace(input.DriveURL) != "" && !validGaspolEvidenceURL(input.DriveURL) {
		return errors.New("INVALID_REPORT_LINK")
	}
	for _, link := range input.EvidenceURLs {
		if !validGaspolEvidenceURL(link) {
			return errors.New("INVALID_REPORT_LINK")
		}
	}
	return nil
}

// Eligibility follows official locks. The downstream lock closes the preceding report.
func (s GaspolService) CanWriteNomination(ctx context.Context, year, schoolID int, kind string) (bool, error) {
	cohort, closing := "NOT_FAILED", "BOTTOM_5"
	if kind == GaspolStageAppeal {
		cohort, closing = "BOTTOM_5", "FINAL"
	} else if kind != GaspolStageNomination {
		return false, errors.New("INVALID_STAGE")
	}
	var eligible bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_cohorts c
 JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED'
 WHERE c.year=$1 AND c.school_id=$2 AND c.cohort=$3 AND c.released_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND lock_type=$4 AND status='LOCKED')
 AND NOT EXISTS(SELECT 1 FROM gaspol_nomination_submissions WHERE year=$1 AND school_id=$2 AND submission_type=$5 AND status='LOCKED')`, year, schoolID, cohort, closing, kind).Scan(&eligible)
	return eligible, err
}

type GaspolNominationReport struct {
	GaspolNominationSubmissionInput
	SchoolID int  `json:"schoolId"`
	CanEdit  bool `json:"canEdit"`
}

func (s GaspolService) NominationReport(ctx context.Context, user auth.User, year, schoolID int, kind string) (GaspolNominationReport, error) {
	result := GaspolNominationReport{GaspolNominationSubmissionInput: GaspolNominationSubmissionInput{Year: year, SubmissionType: kind, EvidenceURLs: []string{}, Status: "DRAFT"}, SchoolID: schoolID}
	if year < 2024 || schoolID <= 0 {
		return result, errors.New("INVALID_PAYLOAD")
	}
	if kind != GaspolStageNomination && kind != GaspolStageAppeal {
		return result, errors.New("INVALID_STAGE")
	}
	own := user.SchoolID != nil && *user.SchoolID == schoolID
	if own {
		if err := s.ensurePhaseSchool(ctx, user, year); err != nil {
			return result, err
		}
		editable, err := s.CanWriteNomination(ctx, year, schoolID, kind)
		if err != nil {
			return result, err
		}
		result.CanEdit = editable
	} else if user.Role != auth.RoleAdminMaster {
		var allowed bool
		err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND school_id=$2 AND assessor_user_id=$3 AND stage=$4 AND is_active AND assignment_role=$5)
  OR EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$3 AND assignment_role='KOOR_GASPOL' AND is_active)
  OR EXISTS(SELECT 1 FROM gaspol_school_participants WHERE year=$1 AND school_id=$2 AND validator_user_id=$3 AND is_active AND NOT is_year_configuration AND EXISTS(SELECT 1 FROM gaspol_user_roles WHERE year=$1 AND user_id=$3 AND role='VALIDATOR' AND is_active))`, year, schoolID, user.ID, kind, stageAssignmentRole(kind)).Scan(&allowed)
		if err != nil {
			return result, err
		}
		if !allowed {
			return result, auth.ErrForbidden
		}
	}
	var evidence []byte
	err := s.DB.Pool.QueryRow(ctx, `SELECT improvement_details,improvement_result,response_text,drive_url,evidence_urls,status FROM gaspol_nomination_submissions WHERE year=$1 AND school_id=$2 AND submission_type=$3`, year, schoolID, kind).Scan(&result.ImprovementDetails, &result.ImprovementResult, &result.ResponseText, &result.DriveURL, &evidence, &result.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return result, err
	}
	if err = json.Unmarshal(evidence, &result.EvidenceURLs); err != nil {
		return result, err
	}
	return result, nil
}
