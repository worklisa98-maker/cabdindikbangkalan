package services

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
)

type GaspolLikeFetchResult struct {
	Count    int
	Platform string
}

// GaspolLikeProvider is deliberately provider-neutral. Production deployments
// must inject an official platform API implementation and credentials; there is
// no HTML scraper fallback.
type GaspolLikeProvider interface {
	TestFetch(context.Context, string) (GaspolLikeFetchResult, error)
}

func DetectGaspolContentPlatform(rawURL string) (string, error) {
	if strings.TrimSpace(rawURL) == "" {
		return "", errors.New("CONTENT_URL_REQUIRED")
	}
	parsed, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil || parsed.Scheme != "https" {
		return "", errors.New("INVALID_CONTENT_URL")
	}
	host := strings.ToLower(parsed.Hostname())
	switch {
	case strings.HasSuffix(host, "instagram.com"):
		return "INSTAGRAM", nil
	case strings.HasSuffix(host, "tiktok.com"):
		return "TIKTOK", nil
	case strings.HasSuffix(host, "youtube.com") || strings.HasSuffix(host, "youtu.be"):
		return "YOUTUBE", nil
	case strings.HasSuffix(host, "facebook.com") || strings.HasSuffix(host, "fb.watch"):
		return "FACEBOOK", nil
	default:
		return "", errors.New("UNSUPPORTED_CONTENT_PLATFORM")
	}
}

func ShouldRunGaspolLikeSync(contentURL, status string, now time.Time, eventStart, cutoff *time.Time, lastSync *time.Time) bool {
	if strings.TrimSpace(contentURL) == "" || strings.ToUpper(status) != "SYNC_ACTIVE" {
		return false
	}
	if cutoff != nil && !now.Before(*cutoff) {
		return true
	}
	interval := 24 * time.Hour
	if eventStart != nil && !now.Before(*eventStart) {
		interval = 30 * time.Minute
	}
	return lastSync == nil || now.Sub(*lastSync) >= interval
}

type GaspolVotingSessionResult struct {
	ID       int64     `json:"id"`
	Token    string    `json:"token"`
	ClosesAt time.Time `json:"closesAt"`
}
type GaspolVoteInput struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	SchoolID int    `json:"schoolId"`
}

type GaspolFavoriteContentInput struct {
	Year       int    `json:"year"`
	SchoolID   int    `json:"schoolId"`
	ContentURL string `json:"contentUrl"`
}

func (s GaspolService) SetFavoriteContent(ctx context.Context, user auth.User, input GaspolFavoriteContentInput, provider GaspolLikeProvider) error {
	year := normalizeGaspolYear(input.Year)
	if !auth.IsAdminMaster(user) {
		if err := s.ensurePhaseSchool(ctx, user, year); err != nil {
			return err
		}
		if input.SchoolID > 0 && input.SchoolID != *user.SchoolID {
			return auth.ErrForbidden
		}
	}

	schoolID := input.SchoolID
	if user.SchoolID != nil && auth.NormalizeRole(user.Role) != auth.RoleAdminMaster {
		schoolID = *user.SchoolID
	}
	if schoolID <= 0 {
		return errors.New("INVALID_SCHOOL_ID")
	}
	var eligible bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.year=c.year AND l.status='LOCKED' WHERE c.year=$1 AND c.school_id=$2 AND c.cohort='TOP_5' AND c.released_at IS NULL)`, year, schoolID).Scan(&eligible)
	if err != nil || !eligible {
		return errors.New("SCHOOL_NOT_ELIGIBLE")
	}
	urlValue := strings.TrimSpace(input.ContentURL)
	if urlValue == "" {
		tag, writeErr := s.DB.Pool.Exec(ctx, `INSERT INTO gaspol_favorite_content(year,school_id,status,updated_by) VALUES($1,$2,'NOT_SUBMITTED',$3) ON CONFLICT(year,school_id) DO UPDATE SET content_url='',platform='',status='NOT_SUBMITTED',last_error='',updated_by=$3,updated_at=NOW() WHERE gaspol_favorite_content.link_locked_at IS NULL`, year, schoolID, user.ID)
		if writeErr != nil {
			return writeErr
		}
		if tag.RowsAffected() == 0 {
			return errors.New("CONTENT_LINK_LOCKED")
		}
		return nil
	}
	platform, err := DetectGaspolContentPlatform(urlValue)
	if err != nil {
		return err
	}
	var lockedAt *time.Time
	_ = s.DB.Pool.QueryRow(ctx, `SELECT link_locked_at FROM gaspol_favorite_content WHERE year=$1 AND school_id=$2`, year, schoolID).Scan(&lockedAt)
	if lockedAt != nil {
		return errors.New("CONTENT_LINK_LOCKED")
	}
	status, lastError, count := "VALIDATING", "", (*int)(nil)
	if provider == nil {
		status = "SYNC_ERROR"
		lastError = "Provider API resmi/credential belum dikonfigurasi"
	} else {
		result, fetchErr := provider.TestFetch(ctx, urlValue)
		if fetchErr != nil {
			status = "SYNC_ERROR"
			lastError = fetchErr.Error()
		} else {
			status = "SYNC_ACTIVE"
			value := result.Count
			count = &value
		}
	}
	tag, err := s.DB.Pool.Exec(ctx, `INSERT INTO gaspol_favorite_content(year,school_id,content_url,platform,status,last_auto_like_count,effective_like_count,last_error,updated_by) VALUES($1,$2,$3,$4,$5,$6,$6,$7,$8) ON CONFLICT(year,school_id) DO UPDATE SET content_url=EXCLUDED.content_url,platform=EXCLUDED.platform,status=EXCLUDED.status,last_auto_like_count=EXCLUDED.last_auto_like_count,effective_like_count=EXCLUDED.effective_like_count,last_error=EXCLUDED.last_error,updated_by=EXCLUDED.updated_by,updated_at=NOW() WHERE gaspol_favorite_content.link_locked_at IS NULL`, year, schoolID, urlValue, platform, status, count, lastError, user.ID)
	if err == nil && tag.RowsAffected() == 0 {
		return errors.New("CONTENT_LINK_LOCKED")
	}
	if err == nil {
		_ = s.Repo.LogAudit(ctx, &year, user.ID, user.Role, "FAVORITE_CONTENT_LINK_TESTED", "favorite_content", nil, &schoolID, map[string]any{"platform": platform, "status": status, "error": lastError})
	}
	return err
}

// Legacy shared-token voting is intentionally retired; all ballots require
// the authenticated application identity and an approved session roster.
func (s GaspolService) OpenVotingSession(ctx context.Context, user auth.User, year, denominator int, closesAt time.Time) (GaspolVotingSessionResult, error) {
	return GaspolVotingSessionResult{}, errors.New("MANAGED_VOTING_REQUIRED")
}
func (s GaspolService) CastFavoriteVote(ctx context.Context, input GaspolVoteInput) error {
	return auth.ErrUnauthorized
}
