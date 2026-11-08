package services

import (
	"cabdin-bangkalan-backend/internal/auth"
	"context"
	"errors"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
)

type GaspolAssistCommand struct {
	Action    string `json:"action"`
	SessionID int64  `json:"sessionId"`
	SchoolID  int    `json:"schoolId"`
	UserID    int    `json:"userId"`
	ID        int64  `json:"id"`
	Reason    string `json:"reason"`
	Active    bool   `json:"active"`
}

func (s GaspolService) VotingAssistance(ctx context.Context, u auth.User, in GaspolAssistCommand) error {
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var year int
	var status string
	if err = tx.QueryRow(ctx, `SELECT year,status FROM gaspol_voting_sessions WHERE id=$1 AND managed FOR UPDATE`, in.SessionID).Scan(&year, &status); err != nil {
		return err
	}
	if in.Action == "ASSIGN" {
		if !auth.IsAdminMaster(u) {
			return auth.ErrForbidden
		}
		if status != "DRAFT" {
			return errors.New("VOTING_ROSTER_LOCKED")
		}
		var valid bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND school_id IS NULL AND role::text IN ('ADMIN','ADMIN_MASTER','PENGAWAS'))`, in.UserID).Scan(&valid); err != nil {
			return err
		}
		if !valid {
			return auth.ErrForbidden
		}
		_, err = tx.Exec(ctx, `INSERT INTO gaspol_voting_operators(session_id,user_id,assigned_by,is_active) VALUES($1,$2,$3,$4) ON CONFLICT(session_id,user_id) DO UPDATE SET is_active=EXCLUDED.is_active,assigned_by=EXCLUDED.assigned_by`, in.SessionID, in.UserID, u.ID, in.Active)
		if err != nil {
			return err
		}
	} else {
		var open bool
		if err = tx.QueryRow(ctx, `SELECT status='OPEN' AND NOW()<closes_at FROM gaspol_voting_sessions WHERE id=$1`, in.SessionID).Scan(&open); err != nil {
			return err
		}
		if !open {
			return errors.New("VOTING_SESSION_CLOSED")
		}
		if in.Action == "REQUEST" {
			if strings.TrimSpace(in.Reason) == "" {
				return errors.New("INVALID_PAYLOAD")
			}
			var valid bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_voting_voters WHERE session_id=$1 AND user_id=$2 AND present AND is_active) AND EXISTS(SELECT 1 FROM gaspol_voting_candidates WHERE session_id=$1 AND school_id=$3) AND NOT EXISTS(SELECT 1 FROM gaspol_internal_votes WHERE session_id=$1 AND voter_user_id=$2)`, in.SessionID, u.ID, in.SchoolID).Scan(&valid)
			if err != nil {
				return err
			}
			if !valid {
				return auth.ErrForbidden
			}
			err = tx.QueryRow(ctx, `INSERT INTO gaspol_vote_assistance(session_id,voter_user_id,school_id,reason) VALUES($1,$2,$3,$4) RETURNING id`, in.SessionID, u.ID, in.SchoolID, in.Reason).Scan(&in.ID)
			if err != nil {
				return err
			}
		} else if in.Action == "EXECUTE" {
			var valid bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_voting_operators WHERE session_id=$1 AND user_id=$2 AND is_active)`, in.SessionID, u.ID).Scan(&valid)
			if err != nil {
				return err
			}
			if !valid {
				return auth.ErrForbidden
			}
			var voter, school int
			err = tx.QueryRow(ctx, `SELECT a.voter_user_id,a.school_id FROM gaspol_vote_assistance a JOIN gaspol_voting_voters v ON v.session_id=a.session_id AND v.user_id=a.voter_user_id AND v.present AND v.is_active WHERE a.id=$1 AND a.session_id=$2 AND a.executed_at IS NULL FOR UPDATE OF a`, in.ID, in.SessionID).Scan(&voter, &school)
			if err != nil {
				return errors.New("ASSISTANCE_CONSENT_REQUIRED")
			}
			_, err = tx.Exec(ctx, `INSERT INTO gaspol_internal_votes(session_id,voter_user_id,school_id) VALUES($1,$2,$3)`, in.SessionID, voter, school)
			if err != nil {
				var pe *pgconn.PgError
				if errors.As(err, &pe) && pe.Code == "23505" {
					return errors.New("VOTE_ALREADY_CAST")
				}
				return err
			}
			if _, err = tx.Exec(ctx, `UPDATE gaspol_vote_assistance SET operator_user_id=$2,executed_at=NOW() WHERE id=$1`, in.ID, u.ID); err != nil {
				return err
			}
			in.UserID = voter
			in.SchoolID = 0
		} else {
			return errors.New("INVALID_PAYLOAD")
		}
	}
	// Candidate is stored in the consent record, scoped to the delegated operator;
	// participation audit does not expose voter choices to other users.
	if err = gaspolAuditTx(ctx, tx, u, year, "VOTING_ASSIST_"+in.Action, "vote_assistance", in.ID, nil, map[string]any{"sessionId": in.SessionID, "voterId": in.UserID, "reason": in.Reason, "active": in.Active}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s GaspolService) AssistanceQueue(ctx context.Context, u auth.User, year int) ([]map[string]any, error) {
	rows, err := s.DB.Pool.Query(ctx, `SELECT a.id,a.session_id,a.voter_user_id,u.username,ds.nama_sekolah,a.reason,a.consent_at::text FROM gaspol_vote_assistance a JOIN gaspol_voting_sessions s ON s.id=a.session_id JOIN users u ON u.id=a.voter_user_id JOIN data_sekolah ds ON ds.id=a.school_id WHERE s.year=$1 AND s.status='OPEN' AND a.executed_at IS NULL AND EXISTS(SELECT 1 FROM gaspol_voting_operators o WHERE o.session_id=a.session_id AND o.user_id=$2 AND o.is_active) ORDER BY a.id`, year, u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, session int64
		var voter int
		var name, school, reason, at string
		if err = rows.Scan(&id, &session, &voter, &name, &school, &reason, &at); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "sessionId": session, "voterId": voter, "voter": name, "school": school, "reason": reason, "consentAt": at})
	}
	return out, rows.Err()
}
