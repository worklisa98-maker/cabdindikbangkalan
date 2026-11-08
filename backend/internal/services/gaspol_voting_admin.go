package services

import (
	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/utils"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"strings"
	"time"
)

type GaspolVotingCommand struct {
	Action    string    `json:"action"`
	Year      int       `json:"year"`
	SessionID int64     `json:"sessionId"`
	ClosesAt  time.Time `json:"closesAt"`
	CutoffAt  time.Time `json:"cutoffAt"`
	Capacity  int       `json:"capacity"`
	UserIDs   []int     `json:"userIds"`
	Present   bool      `json:"present"`
	Active    bool      `json:"active"`
	Accounts  []struct {
		Username string `json:"username"`
		Name     string `json:"name"`
	} `json:"accounts"`
}

func (s GaspolService) ManageVoting(ctx context.Context, u auth.User, in GaspolVotingCommand) (any, error) {
	if err := s.ensureAdminMaster(u); err != nil {
		return nil, err
	}
	if in.Year < 2024 {
		return nil, errors.New("INVALID_YEAR")
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var result any = map[string]bool{"saved": true}
	if in.Action == "CREATE" {
		if !in.ClosesAt.After(time.Now()) || in.CutoffAt.IsZero() {
			return nil, errors.New("INVALID_PAYLOAD")
		}
		if in.Capacity <= 0 {
			in.Capacity = 33
		}
		_, hash, e := GenerateGaspolVotingToken()
		if e != nil {
			return nil, e
		}
		err = tx.QueryRow(ctx, `INSERT INTO gaspol_voting_sessions(year,token_hash,status,voter_denominator,closes_at,created_by,managed) VALUES($1,$2,'DRAFT',$3,$4,$5,TRUE) RETURNING id`, in.Year, hash, in.Capacity, in.ClosesAt, u.ID).Scan(&in.SessionID)
		if err != nil {
			return nil, err
		}
		rows, e := tx.Query(ctx, `SELECT c.school_id FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED' WHERE c.year=$1 AND c.cohort='TOP_5' AND c.released_at IS NULL`, in.Year)
		if e != nil {
			return nil, e
		}
		ids := []int{}
		for rows.Next() {
			var id int
			if e = rows.Scan(&id); e != nil {
				rows.Close()
				return nil, e
			}
			ids = append(ids, id)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		if len(ids) != 5 {
			return nil, errors.New("INCOMPLETE_ASSESSMENTS")
		}
		for _, id := range ids {
			buf := make([]byte, 6)
			if _, err = rand.Read(buf); err != nil {
				return nil, err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO gaspol_voting_candidates(session_id,school_id,short_code) VALUES($1,$2,$3)`, in.SessionID, id, strings.ToUpper(hex.EncodeToString(buf))); err != nil {
				return nil, err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO gaspol_competition_configs(year,september_field1_deadline,like_cutoff_at) VALUES($1,make_date($1,10,15),$2) ON CONFLICT(year) DO UPDATE SET like_cutoff_at=EXCLUDED.like_cutoff_at,updated_by=$3`, in.Year, in.CutoffAt, u.ID)
		if err != nil {
			return nil, err
		}
		result = map[string]any{"id": in.SessionID}
	} else {
		var status string
		var year int
		var closes time.Time
		var capacity int
		err = tx.QueryRow(ctx, `SELECT year,status,closes_at,voter_denominator FROM gaspol_voting_sessions WHERE id=$1 AND managed FOR UPDATE`, in.SessionID).Scan(&year, &status, &closes, &capacity)
		if err != nil {
			return nil, err
		}
		if year != in.Year {
			return nil, auth.ErrForbidden
		}
		switch in.Action {
		case "VOTERS", "ACCOUNTS":
			if status != "DRAFT" {
				return nil, errors.New("VOTING_ROSTER_LOCKED")
			}
			if len(in.UserIDs) > 200 || len(in.Accounts) > 200 {
				return nil, errors.New("INVALID_PAYLOAD")
			}
			credentials := []map[string]any{}
			for _, a := range in.Accounts {
				name := strings.TrimSpace(a.Username)
				if name == "" {
					return nil, errors.New("INVALID_PAYLOAD")
				}
				var id int
				err = tx.QueryRow(ctx, `SELECT id FROM users WHERE LOWER(username)=LOWER($1)`, name).Scan(&id)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return nil, err
				}
				if id == 0 {
					buf := make([]byte, 18)
					if _, err = rand.Read(buf); err != nil {
						return nil, err
					}
					password := hex.EncodeToString(buf)
					hash, e := utils.HashPassword(password)
					if e != nil {
						return nil, e
					}
					err = tx.QueryRow(ctx, `INSERT INTO users(username,password_hash,role,nama_instansi) VALUES($1,$2,'GASPOL_VOTER',$3) RETURNING id`, name, hash, a.Name).Scan(&id)
					if err != nil {
						return nil, err
					}
					credentials = append(credentials, map[string]any{"userId": id, "username": name, "initialPassword": password})
					if err = gaspolAuditTx(ctx, tx, u, in.Year, "VOTER_ACCOUNT_CREATED", "user", int64(id), nil, map[string]any{"username": name}); err != nil {
						return nil, err
					}
				}
				in.UserIDs = append(in.UserIDs, id)
			}
			for _, id := range in.UserIDs {
				var eligible bool
				err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1 AND school_id IS NULL AND role::text IN ('ADMIN','ADMIN_MASTER','PENGAWAS','GASPOL_VOTER'))`, id).Scan(&eligible)
				if err != nil {
					return nil, err
				}
				if !eligible {
					return nil, errors.New("VOTER_NOT_ELIGIBLE")
				}
				_, err = tx.Exec(ctx, `INSERT INTO gaspol_voting_voters(session_id,user_id,present,is_active,assigned_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT(session_id,user_id) DO UPDATE SET present=EXCLUDED.present,is_active=EXCLUDED.is_active,assigned_by=EXCLUDED.assigned_by,updated_at=NOW()`, in.SessionID, id, in.Present, in.Active, u.ID)
				if err != nil {
					return nil, err
				}
			}
			result = map[string]any{"credentials": credentials, "saved": true}
		case "OPEN":
			if status != "DRAFT" || !closes.After(time.Now()) {
				return nil, errors.New("VOTING_SESSION_CLOSED")
			}
			var count int
			err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_voting_voters WHERE session_id=$1 AND present AND is_active`, in.SessionID).Scan(&count)
			if err != nil {
				return nil, err
			}
			if count <= 0 || count > capacity {
				return nil, errors.New("INVALID_VOTER_DENOMINATOR")
			}
			var valid int
			err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM gaspol_voting_candidates vc JOIN gaspol_cohorts c ON c.school_id=vc.school_id AND c.year=$2 AND c.cohort='TOP_5' AND c.released_at IS NULL JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.status='LOCKED' AND l.year=c.year WHERE vc.session_id=$1`, in.SessionID, in.Year).Scan(&valid)
			if err != nil {
				return nil, err
			}
			if valid != 5 {
				return nil, errors.New("SCHOOL_NOT_ELIGIBLE")
			}
			if _, err = tx.Exec(ctx, `UPDATE gaspol_voting_sessions SET status='OPEN',opened_at=NOW(),voter_denominator=$2 WHERE id=$1`, in.SessionID, count); err != nil {
				return nil, err
			}
			_, err = tx.Exec(ctx, `UPDATE gaspol_competition_configs SET internal_voter_denominator=$2,voter_denominator_locked_at=NOW(),like_threshold=800 WHERE year=$1`, in.Year, count)
			if err != nil {
				return nil, err
			}
		case "CLOSE":
			if status != "OPEN" {
				return nil, errors.New("VOTING_SESSION_CLOSED")
			}
			_, err = tx.Exec(ctx, `UPDATE gaspol_voting_sessions SET status='CLOSED',closed_at=NOW() WHERE id=$1`, in.SessionID)
			if err != nil {
				return nil, err
			}
		default:
			return nil, errors.New("INVALID_PAYLOAD")
		}
	}
	// Never include account passwords in the audit payload.
	if err = gaspolAuditTx(ctx, tx, u, in.Year, "VOTING_"+in.Action, "voting_session", in.SessionID, nil, map[string]any{"userIds": in.UserIDs, "present": in.Present, "active": in.Active, "capacity": in.Capacity}); err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}

type GaspolAuthenticatedVote struct {
	SessionID int64  `json:"sessionId"`
	SchoolID  int    `json:"schoolId"`
	ShortCode string `json:"shortCode"`
}

func (s GaspolService) CastAuthenticatedVote(ctx context.Context, u auth.User, in GaspolAuthenticatedVote) error {
	if u.ID <= 0 {
		return auth.ErrUnauthorized
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var year int
	err = tx.QueryRow(ctx, `SELECT year FROM gaspol_voting_sessions WHERE id=$1 AND managed AND status='OPEN' AND NOW()<closes_at FOR UPDATE`, in.SessionID).Scan(&year)
	if err != nil {
		return errors.New("VOTING_SESSION_CLOSED")
	}
	var allowed bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_voting_voters WHERE session_id=$1 AND user_id=$2 AND present AND is_active)`, in.SessionID, u.ID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return auth.ErrForbidden
	}
	var school int
	err = tx.QueryRow(ctx, `SELECT school_id FROM gaspol_voting_candidates WHERE session_id=$1 AND (($2>0 AND school_id=$2) OR ($2=0 AND short_code=$3))`, in.SessionID, in.SchoolID, strings.ToUpper(strings.TrimSpace(in.ShortCode))).Scan(&school)
	if err != nil {
		return errors.New("SCHOOL_NOT_ELIGIBLE")
	}
	_, err = tx.Exec(ctx, `INSERT INTO gaspol_internal_votes(session_id,voter_user_id,school_id) VALUES($1,$2,$3)`, in.SessionID, u.ID, school)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) && pe.Code == "23505" {
			return errors.New("VOTE_ALREADY_CAST")
		}
		return err
	}
	if err = gaspolAuditTx(ctx, tx, u, year, "FAVORITE_VOTE_CAST", "voting_session", in.SessionID, nil, map[string]any{"participated": true}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s GaspolService) VotingView(ctx context.Context, u auth.User, year int) (map[string]any, error) {
	if year < 2024 {
		return nil, errors.New("INVALID_YEAR")
	}
	master := auth.IsAdminMaster(u)
	var id int64
	var status string
	var denominator int
	err := s.DB.Pool.QueryRow(ctx, `SELECT id,status,voter_denominator FROM gaspol_voting_sessions WHERE year=$1 AND managed AND status<>'CANCELLED'`, year).Scan(&id, &status, &denominator)
	if err != nil {
		return nil, err
	}
	var voter, school bool
	if err = s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_voting_voters WHERE session_id=$1 AND user_id=$2 AND is_active)`, id, u.ID).Scan(&voter); err != nil {
		return nil, err
	}
	if u.SchoolID != nil {
		if err = s.ensurePhaseSchool(ctx, u, year); err != nil {
			return nil, err
		}
		if err = s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_voting_candidates WHERE session_id=$1 AND school_id=$2)`, id, *u.SchoolID).Scan(&school); err != nil {
			return nil, err
		}
	}
	if !master && !voter && !school {
		return nil, auth.ErrForbidden
	}
	rows, err := s.DB.Pool.Query(ctx, `SELECT vc.school_id,ds.nama_sekolah,COALESCE(ds.npsn_sekolah,''),vc.short_code FROM gaspol_voting_candidates vc JOIN data_sekolah ds ON ds.id=vc.school_id WHERE vc.session_id=$1 AND ($2 OR $3 OR vc.school_id=$4) ORDER BY ds.nama_sekolah`, id, master, voter, u.SchoolID)
	if err != nil {
		return nil, err
	}
	candidates := []map[string]any{}
	for rows.Next() {
		var sid int
		var name, npsn, code string
		if err = rows.Scan(&sid, &name, &npsn, &code); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, map[string]any{"schoolId": sid, "schoolName": name, "npsn": npsn, "shortCode": code})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	var voted, present bool
	err = s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_internal_votes WHERE session_id=$1 AND voter_user_id=$2),EXISTS(SELECT 1 FROM gaspol_voting_voters WHERE session_id=$1 AND user_id=$2 AND present AND is_active)`, id, u.ID).Scan(&voted, &present)
	if err != nil {
		return nil, err
	}
	out := map[string]any{"id": id, "year": year, "status": status, "candidates": candidates, "voted": voted, "canVote": voter && present && status == "OPEN" && !voted}
	if master {
		rows, err = s.DB.Pool.Query(ctx, `SELECT r.user_id,u.username,r.present,r.is_active,v.id IS NOT NULL FROM gaspol_voting_voters r JOIN users u ON u.id=r.user_id LEFT JOIN gaspol_internal_votes v ON v.session_id=r.session_id AND v.voter_user_id=r.user_id WHERE r.session_id=$1 ORDER BY u.username`, id)
		if err != nil {
			return nil, err
		}
		roster := []map[string]any{}
		attended, cast := 0, 0
		for rows.Next() {
			var uid int
			var name string
			var p, a, v bool
			if err = rows.Scan(&uid, &name, &p, &a, &v); err != nil {
				rows.Close()
				return nil, err
			}
			roster = append(roster, map[string]any{"userId": uid, "username": name, "present": p, "active": a, "voted": v})
			if p && a {
				attended++
			}
			if v {
				cast++
			}
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
		participation := 0.0
		if attended > 0 {
			participation = float64(cast) * 100 / float64(attended)
		}
		out["roster"] = roster
		out["registered"] = len(roster)
		out["present"] = attended
		out["votedCount"] = cast
		out["notVoted"] = attended - cast
		out["participation"] = participation
		out["denominator"] = denominator
		if status == "CLOSED" || status == "ESTABLISHED" {
			rows, err = s.DB.Pool.Query(ctx, `SELECT school_id,COUNT(*) FROM gaspol_internal_votes WHERE session_id=$1 GROUP BY school_id`, id)
			if err != nil {
				return nil, err
			}
			counts := map[int]int{}
			for rows.Next() {
				var sid, n int
				if err = rows.Scan(&sid, &n); err != nil {
					rows.Close()
					return nil, err
				}
				counts[sid] = n
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return nil, err
			}
			out["results"] = counts
			rows, err = s.DB.Pool.Query(ctx, `SELECT vc.school_id,fc.effective_like_count,
              EXISTS(SELECT 1 FROM gaspol_like_snapshots sn WHERE sn.favorite_content_id=fc.id AND sn.like_count=fc.effective_like_count AND ((sn.review_status='VERIFIED' AND sn.observation_at<=cc.like_cutoff_at) OR (sn.source='AUTO' AND sn.fetch_status='FINAL' AND sn.captured_at<=cc.like_cutoff_at))),
              cc.like_cutoff_at IS NOT NULL AND cc.like_cutoff_at<=NOW()
              FROM gaspol_voting_candidates vc JOIN gaspol_competition_configs cc ON cc.year=$2 LEFT JOIN gaspol_favorite_content fc ON fc.year=$2 AND fc.school_id=vc.school_id WHERE vc.session_id=$1`, id, year)
			if err != nil {
				return nil, err
			}
			preview := []map[string]any{}
			ready := true
			for rows.Next() {
				var sid int
				var likes *int
				var verified, cutoffReached bool
				if err = rows.Scan(&sid, &likes, &verified, &cutoffReached); err != nil {
					rows.Close()
					return nil, err
				}
				n := 0
				if likes != nil {
					n = *likes
				}
				eligible := verified && likes != nil && cutoffReached
				ready = ready && eligible
				preview = append(preview, map[string]any{"schoolId": sid, "likes": likes, "verified": verified, "eligible": eligible, "score": ComputeGaspolFavoriteScore(counts[sid], denominator, n, 800)})
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return nil, err
			}
			out["preview"] = preview
			out["readyToEstablish"] = ready && len(preview) == 5 && status == "CLOSED"

		}
	}
	return out, nil
}
