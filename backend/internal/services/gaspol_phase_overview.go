package services

import (
	"cabdin-bangkalan-backend/internal/auth"
	"context"
	"encoding/json"
)

func (s GaspolService) AssessmentDetails(ctx context.Context, u auth.User, id int64) (map[string]any, error) {
	var year, school, assessor int
	var stage, status, notes string
	var raw []byte
	var score float64
	err := s.DB.Pool.QueryRow(ctx, `SELECT year,school_id,assessor_user_id,stage,workflow_status,raw_score,rubric,notes FROM gaspol_stage_assessments WHERE id=$1`, id).Scan(&year, &school, &assessor, &stage, &status, &score, &raw, &notes)
	if err != nil {
		return nil, err
	}
	if err = s.phaseReadAllowed(ctx, u, year, school, stage); err != nil {
		return nil, err
	}
	var rubric any
	if err = json.Unmarshal(raw, &rubric); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "year": year, "schoolId": school, "assessorUserId": assessor, "stage": stage, "status": status, "score": score, "rubric": rubric, "notes": notes}, nil
}

func (s GaspolService) PhaseOverview(ctx context.Context, u auth.User, year int) (map[string]any, error) {
	out := map[string]any{}
	rows, err := s.DB.Pool.Query(ctx, `SELECT v.school_id,ds.nama_sekolah FROM gaspol_like_verifiers v JOIN data_sekolah ds ON ds.id=v.school_id WHERE v.year=$1 AND v.user_id=$2 AND v.is_active ORDER BY ds.nama_sekolah`, year, u.ID)
	if err != nil {
		return nil, err
	}
	schools := []map[string]any{}
	for rows.Next() {
		var id int
		var name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return nil, err
		}
		schools = append(schools, map[string]any{"schoolId": id, "schoolName": name})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out["likeSchools"] = schools
	var cutoff any
	var cutoffText *string
	if err = s.DB.Pool.QueryRow(ctx, `SELECT like_cutoff_at::text FROM gaspol_competition_configs WHERE year=$1`, year).Scan(&cutoffText); err == nil {
		cutoff = cutoffText
	}
	out["cutoffAt"] = cutoff
	if !auth.IsAdminMaster(u) {
		return out, nil
	}
	rows, err = s.DB.Pool.Query(ctx, `SELECT a.id,a.school_id,ds.nama_sekolah,a.stage,a.assessor_user_id,COALESCE(u.username,''),a.raw_score,a.final_score,a.workflow_status,a.rubric,a.notes FROM gaspol_stage_assessments a JOIN data_sekolah ds ON ds.id=a.school_id JOIN users u ON u.id=a.assessor_user_id WHERE a.year=$1 ORDER BY ds.nama_sekolah,a.stage,a.id`, year)
	if err != nil {
		return nil, err
	}
	assessments := []map[string]any{}
	for rows.Next() {
		var id int64
		var school, actor int
		var name, stage, username, status, notes string
		var rawScore float64
		var final *float64
		var rubric []byte
		if err = rows.Scan(&id, &school, &name, &stage, &actor, &username, &rawScore, &final, &status, &rubric, &notes); err != nil {
			rows.Close()
			return nil, err
		}
		var detail any
		if err = json.Unmarshal(rubric, &detail); err != nil {
			rows.Close()
			return nil, err
		}
		assessments = append(assessments, map[string]any{"id": id, "schoolId": school, "schoolName": name, "stage": stage, "assessor": username, "rawScore": rawScore, "finalScore": final, "status": status, "rubric": detail, "notes": notes, "eligible": status == "FINAL" && final != nil})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out["assessments"] = assessments
	rows, err = s.DB.Pool.Query(ctx, `SELECT a.actor_user_id,COALESCE(u.username,''),a.action,a.entity_type,a.entity_id,a.school_id,a.payload,a.created_at::text FROM gaspol_audit_logs a LEFT JOIN users u ON u.id=a.actor_user_id WHERE a.year=$1 ORDER BY a.created_at DESC,a.id DESC LIMIT 200`, year)
	if err != nil {
		return nil, err
	}
	events := []map[string]any{}
	for rows.Next() {
		var actor *int
		var name, action, entity, at string
		var id *int64
		var school *int
		var raw []byte
		if err = rows.Scan(&actor, &name, &action, &entity, &id, &school, &raw, &at); err != nil {
			rows.Close()
			return nil, err
		}
		var payload any
		if err = json.Unmarshal(raw, &payload); err != nil {
			rows.Close()
			return nil, err
		}
		events = append(events, map[string]any{"actor": name, "actorId": actor, "action": action, "entity": entity, "entityId": id, "schoolId": school, "payload": payload, "at": at})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out["audit"] = events
	return out, nil
}
