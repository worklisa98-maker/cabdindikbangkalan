package services

import (
	"cabdin-bangkalan-backend/internal/auth"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math"
	"strings"
)

type GaspolIndicator struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Weight   float64 `json:"weight"`
	Guidance string  `json:"guidance"`
}
type GaspolRubricVersion struct {
	ID          int64             `json:"id"`
	Year        int               `json:"year"`
	Stage       string            `json:"stage"`
	Version     int               `json:"version"`
	Title       string            `json:"title"`
	Guidance    string            `json:"guidance"`
	Provisional bool              `json:"provisional"`
	Indicators  []GaspolIndicator `json:"indicators"`
	Status      string            `json:"status"`
}

func ValidateGaspolRubric(items []GaspolIndicator, activate bool) error {
	if len(items) == 0 {
		return errors.New("RUBRIC_INDICATORS_REQUIRED")
	}
	total := 0.0
	keys := map[string]bool{}
	for _, i := range items {
		if strings.TrimSpace(i.Key) == "" || strings.TrimSpace(i.Label) == "" || keys[i.Key] || math.IsNaN(i.Weight) || math.IsInf(i.Weight, 0) || i.Weight < 0 || i.Weight > 100 {
			return errors.New("INVALID_RUBRIC")
		}
		keys[i.Key] = true
		total += i.Weight
	}
	if activate && math.Abs(total-100) > 0.000001 {
		return errors.New("RUBRIC_WEIGHT_MUST_TOTAL_100")
	}
	return nil
}
func ComputeGaspolRubric(items []GaspolIndicator, scores map[string]float64) (float64, error) {
	if err := ValidateGaspolRubric(items, true); err != nil {
		return 0, err
	}
	if len(scores) != len(items) {
		return 0, errors.New("RUBRIC_SCORES_REQUIRED")
	}
	total := 0.0
	for _, i := range items {
		v, ok := scores[i.Key]
		if !ok || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 || v > 100 {
			return 0, errors.New("INVALID_RUBRIC_SCORE")
		}
		total += v * i.Weight / 100
	}
	return roundGaspolScore(total), nil
}
func (s GaspolService) SaveRubricVersion(ctx context.Context, u auth.User, r GaspolRubricVersion) (int64, error) {
	if err := s.ensureAdminMaster(u); err != nil {
		return 0, err
	}
	if r.Year < 2024 || (r.Stage != GaspolStageNomination && r.Stage != "INNOVATION") || strings.TrimSpace(r.Title) == "" {
		return 0, errors.New("INVALID_PAYLOAD")
	}
	if err := ValidateGaspolRubric(r.Indicators, false); err != nil {
		return 0, err
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("rubric:%d:%s", r.Year, r.Stage)); err != nil {
		return 0, err
	}
	raw, _ := json.Marshal(r.Indicators)
	var id int64
	err = tx.QueryRow(ctx, `INSERT INTO gaspol_rubric_versions(year,stage,version,title,guidance,provisional,indicators,created_by) SELECT $1,$2,COALESCE(MAX(version),0)+1,$3,$4,$5,$6::jsonb,$7 FROM gaspol_rubric_versions WHERE year=$1 AND stage=$2 RETURNING id`, r.Year, r.Stage, r.Title, r.Guidance, r.Provisional, string(raw), u.ID).Scan(&id)
	if err != nil {
		return 0, err
	}
	if err = gaspolAuditTx(ctx, tx, u, r.Year, "RUBRIC_VERSION_CREATED", "rubric", id, nil, r); err != nil {
		return 0, err
	}
	return id, tx.Commit(ctx)
}
func gaspolAuditTx(ctx context.Context, tx pgx.Tx, u auth.User, year int, action, entity string, id int64, school *int, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO gaspol_audit_logs(year,actor_user_id,actor_role,action,entity_type,entity_id,school_id,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8::jsonb)`, year, u.ID, u.Role, action, entity, id, school, string(raw))
	return err
}
func (s GaspolService) ActivateRubric(ctx context.Context, u auth.User, id int64) error {
	if err := s.ensureAdminMaster(u); err != nil {
		return err
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var year int
	var stage string
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT year,stage,indicators FROM gaspol_rubric_versions WHERE id=$1 AND status='DRAFT'`, id).Scan(&year, &stage, &raw); err != nil {
		return errors.New("RUBRIC_NOT_DRAFT")
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("rubric:%d:%s", year, stage)); err != nil {
		return err
	}
	var items []GaspolIndicator
	if err = json.Unmarshal(raw, &items); err != nil {
		return err
	}
	if err = ValidateGaspolRubric(items, true); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE gaspol_rubric_versions SET status='ARCHIVED' WHERE year=$1 AND stage=$2 AND status='ACTIVE'`, year, stage); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE gaspol_rubric_versions SET status='ACTIVE',activated_at=NOW() WHERE id=$1 AND status='DRAFT'`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("RUBRIC_NOT_DRAFT")
	}
	if err = gaspolAuditTx(ctx, tx, u, year, "RUBRIC_ACTIVATED", "rubric", id, nil, map[string]any{"existingBindingsUnchanged": true}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s GaspolService) RubricVersions(ctx context.Context, u auth.User, year int) ([]GaspolRubricVersion, error) {
	if err := s.ensureAdminMaster(u); err != nil {
		return nil, err
	}
	rows, err := s.DB.Pool.Query(ctx, `SELECT id,year,stage,version,title,guidance,provisional,indicators,status FROM gaspol_rubric_versions WHERE year=$1 ORDER BY stage,version DESC`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GaspolRubricVersion{}
	for rows.Next() {
		var r GaspolRubricVersion
		var raw []byte
		if err = rows.Scan(&r.ID, &r.Year, &r.Stage, &r.Version, &r.Title, &r.Guidance, &r.Provisional, &raw, &r.Status); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r.Indicators); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Opening the assessment pins a rubric for this school and stage. Activation
// only affects schools whose assessment has not started.
func (s GaspolService) AssessmentRubric(ctx context.Context, u auth.User, year, school int, stage string) (GaspolRubricVersion, error) {
	stage = normalizeCompetitionStage(stage)
	if year < 2024 || (stage != GaspolStageNomination && stage != "INNOVATION") {
		return GaspolRubricVersion{}, errors.New("INVALID_PAYLOAD")
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return GaspolRubricVersion{}, err
	}
	defer tx.Rollback(ctx)
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND stage=$2 AND school_id=$3 AND assessor_user_id=$4 AND assignment_role=$5 AND is_active)`, year, stage, school, u.ID, stageAssignmentRole(stage)).Scan(&allowed)
	if err != nil {
		return GaspolRubricVersion{}, err
	}
	if !allowed {
		return GaspolRubricVersion{}, auth.ErrForbidden
	}
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, fmt.Sprintf("rubric:%d:%s", year, stage)); err != nil {
		return GaspolRubricVersion{}, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO gaspol_rubric_bindings(year,stage,school_id,rubric_id,bound_by) SELECT $1,$2,$3,id,$4 FROM gaspol_rubric_versions WHERE year=$1 AND stage=$2 AND status='ACTIVE' ON CONFLICT DO NOTHING`, year, stage, school, u.ID)
	if err != nil {
		return GaspolRubricVersion{}, err
	}
	var r GaspolRubricVersion
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT r.id,r.year,r.stage,r.version,r.title,r.guidance,r.provisional,r.indicators,r.status FROM gaspol_rubric_bindings b JOIN gaspol_rubric_versions r ON r.id=b.rubric_id WHERE b.year=$1 AND b.stage=$2 AND b.school_id=$3`, year, stage, school).Scan(&r.ID, &r.Year, &r.Stage, &r.Version, &r.Title, &r.Guidance, &r.Provisional, &raw, &r.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, errors.New("ACTIVE_RUBRIC_REQUIRED")
	}
	if err != nil {
		return r, err
	}
	if err = json.Unmarshal(raw, &r.Indicators); err != nil {
		return r, err
	}
	if tag.RowsAffected() > 0 {
		if err = gaspolAuditTx(ctx, tx, u, year, "RUBRIC_BOUND", "rubric", r.ID, &school, map[string]any{"stage": stage, "version": r.Version}); err != nil {
			return r, err
		}
	}
	return r, tx.Commit(ctx)
}
