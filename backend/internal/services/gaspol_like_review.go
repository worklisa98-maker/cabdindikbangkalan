package services

import (
	"cabdin-bangkalan-backend/internal/auth"
	"context"
	"errors"
	"strings"
	"time"
)

type GaspolLikeCommand struct {
	Action      string    `json:"action"`
	Year        int       `json:"year"`
	SchoolID    int       `json:"schoolId"`
	UserID      int       `json:"userId"`
	ID          int64     `json:"id"`
	SchoolIDs   []int     `json:"schoolIds"`
	Active      bool      `json:"active"`
	LikeCount   int       `json:"likeCount"`
	ContentURL  string    `json:"contentUrl"`
	EvidenceURL string    `json:"evidenceUrl"`
	Reason      string    `json:"reason"`
	CapturedAt  time.Time `json:"capturedAt"`
}

func (s GaspolService) ManageLikes(ctx context.Context, u auth.User, in GaspolLikeCommand) error {
	if in.Year < 2024 {
		return errors.New("INVALID_YEAR")
	}
	master := auth.IsAdminMaster(u)
	if in.Action == "ASSIGN" || in.Action == "VERIFY" || in.Action == "RETURN" {
		if !master {
			return auth.ErrForbidden
		}
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if in.Action == "ASSIGN" {
		if in.UserID <= 0 || len(in.SchoolIDs) == 0 || len(in.SchoolIDs) > 5 {
			return errors.New("INVALID_PAYLOAD")
		}
		for _, school := range in.SchoolIDs {
			var ok bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.status='LOCKED' WHERE c.year=$1 AND c.school_id=$2 AND c.cohort='TOP_5' AND c.released_at IS NULL) AND EXISTS(SELECT 1 FROM users WHERE id=$3 AND school_id IS NULL)`, in.Year, school, in.UserID).Scan(&ok)
			if err != nil {
				return err
			}
			if !ok {
				return errors.New("SCHOOL_NOT_ELIGIBLE")
			}
			if _, err = tx.Exec(ctx, `INSERT INTO gaspol_like_verifiers(year,school_id,user_id,is_active,assigned_by) VALUES($1,$2,$3,$4,$5) ON CONFLICT(year,school_id,user_id) DO UPDATE SET is_active=EXCLUDED.is_active,assigned_by=EXCLUDED.assigned_by,updated_at=NOW()`, in.Year, school, in.UserID, in.Active, u.ID); err != nil {
				return err
			}
		}
	} else if in.Action == "DRAFT" || in.Action == "SUBMIT" {
		var allowed bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_like_verifiers WHERE year=$1 AND school_id=$2 AND user_id=$3 AND is_active)`, in.Year, in.SchoolID, u.ID).Scan(&allowed)
		if err != nil {
			return err
		}
		if !master && !allowed {
			return auth.ErrForbidden
		}
		if in.LikeCount < 0 || !validGaspolEvidenceURL(in.ContentURL) || !validGaspolEvidenceURL(in.EvidenceURL) || strings.TrimSpace(in.Reason) == "" || in.CapturedAt.IsZero() || in.CapturedAt.After(time.Now()) {
			return errors.New("INVALID_PAYLOAD")
		}
		var cutoff *time.Time
		var eligible bool
		err = tx.QueryRow(ctx, `SELECT like_cutoff_at,EXISTS(SELECT 1 FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.status='LOCKED' WHERE c.year=$1 AND c.school_id=$2 AND c.cohort='TOP_5' AND c.released_at IS NULL) AND NOT EXISTS(SELECT 1 FROM gaspol_awards WHERE year=$1 AND award_type='FAVORITE') FROM gaspol_competition_configs WHERE year=$1 FOR UPDATE`, in.Year, in.SchoolID).Scan(&cutoff, &eligible)
		if err != nil {
			return err
		}
		if !eligible {
			return errors.New("SCHOOL_NOT_ELIGIBLE")
		}
		if cutoff == nil || in.CapturedAt.After(*cutoff) {
			return errors.New("LIKE_CAPTURE_AFTER_CUTOFF")
		}
		var contentID int64
		err = tx.QueryRow(ctx, `INSERT INTO gaspol_favorite_content(year,school_id,updated_by) VALUES($1,$2,$3) ON CONFLICT(year,school_id) DO UPDATE SET updated_by=EXCLUDED.updated_by WHERE gaspol_favorite_content.frozen_at IS NULL RETURNING id`, in.Year, in.SchoolID, u.ID).Scan(&contentID)
		if err != nil {
			return err
		}
		status := "DRAFT"
		if in.Action == "SUBMIT" {
			status = "SUBMITTED"
		}
		err = tx.QueryRow(ctx, `INSERT INTO gaspol_like_snapshots(favorite_content_id,like_count,source,fetch_status,reason,evidence_url,actor_user_id,review_status,content_url,observation_at) VALUES($1,$2,'MANUAL','OVERRIDE',$3,$4,$5,$6,$7,$8) RETURNING id`, contentID, in.LikeCount, in.Reason, in.EvidenceURL, u.ID, status, in.ContentURL, in.CapturedAt).Scan(&in.ID)
		if err != nil {
			return err
		}
	} else if in.Action == "VERIFY" || in.Action == "RETURN" {
		if in.Action == "RETURN" && strings.TrimSpace(in.Reason) == "" {
			return errors.New("INVALID_PAYLOAD")
		}
		var count int
		var contentID int64
		var captured time.Time
		var cutoff *time.Time
		var url string
		err = tx.QueryRow(ctx, `SELECT sn.like_count,sn.favorite_content_id,fc.school_id,sn.observation_at,cc.like_cutoff_at,sn.content_url FROM gaspol_like_snapshots sn JOIN gaspol_favorite_content fc ON fc.id=sn.favorite_content_id JOIN gaspol_competition_configs cc ON cc.year=fc.year WHERE sn.id=$1 AND fc.year=$2 AND sn.review_status='SUBMITTED' AND fc.frozen_at IS NULL FOR UPDATE OF sn,fc,cc`, in.ID, in.Year).Scan(&count, &contentID, &in.SchoolID, &captured, &cutoff, &url)
		if err != nil {
			return errors.New("LIKE_NOT_SUBMITTED")
		}
		status := "RETURNED"
		if in.Action == "VERIFY" {
			if cutoff == nil || captured.After(*cutoff) {
				return errors.New("LIKE_CAPTURE_AFTER_CUTOFF")
			}
			status = "VERIFIED"
			_, err = tx.Exec(ctx, `UPDATE gaspol_favorite_content SET effective_like_count=$2,content_url=$3,updated_by=$4,updated_at=NOW(),link_locked_at=COALESCE(link_locked_at,NOW()) WHERE id=$1`, contentID, count, url, u.ID)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE gaspol_like_snapshots SET review_status=$2,reviewed_by=$3,reviewed_at=NOW(),review_notes=$4 WHERE id=$1`, in.ID, status, u.ID, in.Reason)
		if err != nil {
			return err
		}
	} else {
		return errors.New("INVALID_PAYLOAD")
	}
	if err = gaspolAuditTx(ctx, tx, u, in.Year, "LIKE_"+in.Action, "like_snapshot", in.ID, &in.SchoolID, in); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s GaspolService) LikeReviewView(ctx context.Context, u auth.User, year int) ([]map[string]any, error) {
	rows, err := s.DB.Pool.Query(ctx, `SELECT sn.id,fc.school_id,ds.nama_sekolah,sn.like_count,sn.review_status,sn.reason,sn.evidence_url,sn.content_url,sn.observation_at,sn.review_notes,sn.actor_user_id FROM gaspol_like_snapshots sn JOIN gaspol_favorite_content fc ON fc.id=sn.favorite_content_id JOIN data_sekolah ds ON ds.id=fc.school_id WHERE fc.year=$1 AND sn.review_status IS NOT NULL AND ($2 OR EXISTS(SELECT 1 FROM gaspol_like_verifiers v WHERE v.year=fc.year AND v.school_id=fc.school_id AND v.user_id=$3 AND v.is_active)) ORDER BY sn.id DESC`, year, auth.IsAdminMaster(u), u.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var school, count, actor int
		var name, status, reason, evidence, url, notes string
		var at time.Time
		if err = rows.Scan(&id, &school, &name, &count, &status, &reason, &evidence, &url, &at, &notes, &actor); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"id": id, "schoolId": school, "schoolName": name, "likeCount": count, "status": status, "reason": reason, "evidenceUrl": evidence, "contentUrl": url, "capturedAt": at, "reviewNotes": notes, "actorUserId": actor})
	}
	return out, rows.Err()
}
