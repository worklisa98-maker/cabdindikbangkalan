package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
	"cabdin-bangkalan-backend/internal/config"
	"cabdin-bangkalan-backend/internal/db"
)

const systemTimeOverrideKey = "GLOBAL"

var jakartaLocationOnce sync.Once
var jakartaLocation *time.Location

type SystemClock struct {
	DB     *db.DB
	Config config.Config
}

type SystemTimeOverrideRecord struct {
	OverrideKey           string
	IsEnabled             bool
	OverrideBaseTime      *time.Time
	OverrideStartedRealAt *time.Time
	UpdatedBy             *int
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type SystemTimeAuditEntry struct {
	ID                    int         `json:"id"`
	Action                string      `json:"action"`
	ActorUserID           *int        `json:"actorUserId,omitempty"`
	ActorName             string      `json:"actorName"`
	ActorRole             string      `json:"actorRole"`
	OverrideBaseTime      *time.Time  `json:"overrideBaseTime,omitempty"`
	OverrideStartedRealAt *time.Time  `json:"overrideStartedRealAt,omitempty"`
	Payload               interface{} `json:"payload,omitempty"`
	CreatedAt             time.Time   `json:"createdAt"`
}

type SystemTimeStatus struct {
	FeatureEnabled        bool                   `json:"featureEnabled"`
	CanManage             bool                   `json:"canManage"`
	UsingOverride         bool                   `json:"usingOverride"`
	Timezone              string                 `json:"timezone"`
	RealTime              time.Time              `json:"realTime"`
	EffectiveTime         time.Time              `json:"effectiveTime"`
	OverrideBaseTime      *time.Time             `json:"overrideBaseTime,omitempty"`
	OverrideStartedRealAt *time.Time             `json:"overrideStartedRealAt,omitempty"`
	UpdatedBy             *int                   `json:"updatedBy,omitempty"`
	UpdatedAt             *time.Time             `json:"updatedAt,omitempty"`
	RecentAudits          []SystemTimeAuditEntry `json:"recentAudits,omitempty"`
}

func NewSystemClock(database *db.DB, cfg config.Config) SystemClock {
	return SystemClock{
		DB:     database,
		Config: cfg,
	}
}

func JakartaLocation() *time.Location {
	jakartaLocationOnce.Do(func() {
		loc, err := time.LoadLocation("Asia/Jakarta")
		if err != nil {
			loc = time.FixedZone("WIB", 7*60*60)
		}
		jakartaLocation = loc
	})
	return jakartaLocation
}

func normalizeJakartaTime(value time.Time) time.Time {
	return value.In(JakartaLocation())
}

func (c SystemClock) FeatureEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(c.Config.AppEnv), "staging") && c.Config.SystemTimeOverrideEnabled
}

func (c SystemClock) RealNow() time.Time {
	return normalizeJakartaTime(time.Now())
}

func (c SystemClock) Now(ctx context.Context) (time.Time, error) {
	status, err := c.Status(ctx, false, false)
	if err != nil {
		return time.Time{}, err
	}
	return status.EffectiveTime, nil
}

func (c SystemClock) Status(ctx context.Context, canManage bool, includeAudit bool) (SystemTimeStatus, error) {
	realNow := c.RealNow()
	status := SystemTimeStatus{
		FeatureEnabled: c.FeatureEnabled(),
		CanManage:      canManage && c.FeatureEnabled(),
		UsingOverride:  false,
		Timezone:       "Asia/Jakarta",
		RealTime:       realNow,
		EffectiveTime:  realNow,
	}
	if !status.FeatureEnabled {
		return status, nil
	}

	record, err := c.loadOverrideRecord(ctx)
	if err != nil {
		return SystemTimeStatus{}, err
	}
	if record != nil {
		status.OverrideBaseTime = record.OverrideBaseTime
		status.OverrideStartedRealAt = record.OverrideStartedRealAt
		status.UpdatedBy = record.UpdatedBy
		status.UpdatedAt = &record.UpdatedAt
		status.UsingOverride = record.IsEnabled && record.OverrideBaseTime != nil && record.OverrideStartedRealAt != nil
		if status.UsingOverride {
			status.EffectiveTime = CalculateEffectiveSystemTime(*record.OverrideBaseTime, *record.OverrideStartedRealAt, realNow)
		}
	}

	if includeAudit {
		audits, err := c.ListRecentAudits(ctx, 20)
		if err != nil {
			return SystemTimeStatus{}, err
		}
		status.RecentAudits = audits
	}

	return status, nil
}

func (c SystemClock) SetOverride(ctx context.Context, actor auth.User, overrideDatetimeRaw string) (SystemTimeStatus, error) {
	if !c.FeatureEnabled() {
		return SystemTimeStatus{}, auth.ErrForbidden
	}
	if !auth.IsAdminMaster(actor) {
		return SystemTimeStatus{}, auth.ErrForbidden
	}
	overrideBaseTime, err := ParseSystemTimeOverrideInput(overrideDatetimeRaw)
	if err != nil {
		return SystemTimeStatus{}, err
	}
	realNow := c.RealNow()
	payload := map[string]string{
		"overrideDatetime": normalizeJakartaTime(overrideBaseTime).Format(time.RFC3339),
	}
	if err := c.upsertOverrideRecord(ctx, SystemTimeOverrideRecord{
		OverrideKey:           systemTimeOverrideKey,
		IsEnabled:             true,
		OverrideBaseTime:      pointerToTime(normalizeJakartaTime(overrideBaseTime)),
		OverrideStartedRealAt: pointerToTime(realNow),
		UpdatedBy:             pointerToInt(actor.ID),
		CreatedAt:             realNow,
		UpdatedAt:             realNow,
	}); err != nil {
		return SystemTimeStatus{}, err
	}
	if err := c.insertAudit(ctx, "SET_OVERRIDE", actor, pointerToTime(normalizeJakartaTime(overrideBaseTime)), pointerToTime(realNow), payload); err != nil {
		return SystemTimeStatus{}, err
	}
	return c.Status(ctx, true, true)
}

func (c SystemClock) ResetOverride(ctx context.Context, actor auth.User) (SystemTimeStatus, error) {
	if !c.FeatureEnabled() {
		return SystemTimeStatus{}, auth.ErrForbidden
	}
	if !auth.IsAdminMaster(actor) {
		return SystemTimeStatus{}, auth.ErrForbidden
	}
	realNow := c.RealNow()
	previous, err := c.loadOverrideRecord(ctx)
	if err != nil {
		return SystemTimeStatus{}, err
	}
	payload := map[string]string{}
	if previous != nil && previous.OverrideBaseTime != nil {
		payload["previousOverrideBaseTime"] = normalizeJakartaTime(*previous.OverrideBaseTime).Format(time.RFC3339)
	}
	if previous != nil && previous.OverrideStartedRealAt != nil {
		payload["previousOverrideStartedRealAt"] = normalizeJakartaTime(*previous.OverrideStartedRealAt).Format(time.RFC3339)
	}
	if err := c.upsertOverrideRecord(ctx, SystemTimeOverrideRecord{
		OverrideKey:           systemTimeOverrideKey,
		IsEnabled:             false,
		OverrideBaseTime:      nil,
		OverrideStartedRealAt: nil,
		UpdatedBy:             pointerToInt(actor.ID),
		CreatedAt:             realNow,
		UpdatedAt:             realNow,
	}); err != nil {
		return SystemTimeStatus{}, err
	}
	if err := c.insertAudit(ctx, "RESET_TO_REAL", actor, nil, nil, payload); err != nil {
		return SystemTimeStatus{}, err
	}
	return c.Status(ctx, true, true)
}

func (c SystemClock) ListRecentAudits(ctx context.Context, limit int) ([]SystemTimeAuditEntry, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := c.DB.Pool.Query(ctx, `
		SELECT sta.id,
		       sta.action,
		       sta.actor_user_id,
		       COALESCE(NULLIF(TRIM(u.nama_instansi), ''), NULLIF(TRIM(u.username), ''), '') AS actor_name,
		       sta.actor_role,
		       sta.override_base_time,
		       sta.override_started_real_at,
		       sta.payload,
		       sta.created_at
		FROM system_time_override_audit_logs sta
		LEFT JOIN users u ON u.id = sta.actor_user_id
		ORDER BY sta.created_at DESC, sta.id DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]SystemTimeAuditEntry, 0)
	for rows.Next() {
		var item SystemTimeAuditEntry
		var payloadRaw []byte
		if err := rows.Scan(
			&item.ID,
			&item.Action,
			&item.ActorUserID,
			&item.ActorName,
			&item.ActorRole,
			&item.OverrideBaseTime,
			&item.OverrideStartedRealAt,
			&payloadRaw,
			&item.CreatedAt,
		); err != nil {
			return nil, err
		}
		item.CreatedAt = normalizeJakartaTime(item.CreatedAt)
		if item.OverrideBaseTime != nil {
			normalized := normalizeJakartaTime(*item.OverrideBaseTime)
			item.OverrideBaseTime = &normalized
		}
		if item.OverrideStartedRealAt != nil {
			normalized := normalizeJakartaTime(*item.OverrideStartedRealAt)
			item.OverrideStartedRealAt = &normalized
		}
		if len(payloadRaw) > 0 {
			var payload interface{}
			if err := json.Unmarshal(payloadRaw, &payload); err == nil {
				item.Payload = payload
			}
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func ParseSystemTimeOverrideInput(raw string) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, errors.New("INVALID_OVERRIDE_DATETIME")
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if layout == time.RFC3339 {
			parsed, err := time.Parse(layout, value)
			if err == nil {
				return normalizeJakartaTime(parsed), nil
			}
			continue
		}
		parsed, err := time.ParseInLocation(layout, value, JakartaLocation())
		if err == nil {
			return normalizeJakartaTime(parsed), nil
		}
	}
	return time.Time{}, errors.New("INVALID_OVERRIDE_DATETIME")
}

func (c SystemClock) loadOverrideRecord(ctx context.Context) (*SystemTimeOverrideRecord, error) {
	var item SystemTimeOverrideRecord
	err := c.DB.Pool.QueryRow(ctx, `
		SELECT override_key,
		       is_enabled,
		       override_base_time,
		       override_started_real_at,
		       updated_by,
		       created_at,
		       updated_at
		FROM system_time_overrides
		WHERE override_key = $1
	`, systemTimeOverrideKey).Scan(
		&item.OverrideKey,
		&item.IsEnabled,
		&item.OverrideBaseTime,
		&item.OverrideStartedRealAt,
		&item.UpdatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if item.OverrideBaseTime != nil {
		normalized := normalizeJakartaTime(*item.OverrideBaseTime)
		item.OverrideBaseTime = &normalized
	}
	if item.OverrideStartedRealAt != nil {
		normalized := normalizeJakartaTime(*item.OverrideStartedRealAt)
		item.OverrideStartedRealAt = &normalized
	}
	item.CreatedAt = normalizeJakartaTime(item.CreatedAt)
	item.UpdatedAt = normalizeJakartaTime(item.UpdatedAt)
	return &item, nil
}

func (c SystemClock) upsertOverrideRecord(ctx context.Context, record SystemTimeOverrideRecord) error {
	_, err := c.DB.Pool.Exec(ctx, `
		INSERT INTO system_time_overrides (
			override_key,
			is_enabled,
			override_base_time,
			override_started_real_at,
			updated_by,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (override_key) DO UPDATE SET
			is_enabled = EXCLUDED.is_enabled,
			override_base_time = EXCLUDED.override_base_time,
			override_started_real_at = EXCLUDED.override_started_real_at,
			updated_by = EXCLUDED.updated_by,
			updated_at = EXCLUDED.updated_at
	`, record.OverrideKey, record.IsEnabled, record.OverrideBaseTime, record.OverrideStartedRealAt, record.UpdatedBy, record.CreatedAt, record.UpdatedAt)
	return err
}

func (c SystemClock) insertAudit(ctx context.Context, action string, actor auth.User, overrideBaseTime *time.Time, overrideStartedRealAt *time.Time, payload interface{}) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = c.DB.Pool.Exec(ctx, `
		INSERT INTO system_time_override_audit_logs (
			action,
			actor_user_id,
			actor_role,
			override_base_time,
			override_started_real_at,
			payload,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, strings.TrimSpace(action), actor.ID, auth.NormalizeRole(actor.Role), overrideBaseTime, overrideStartedRealAt, payloadJSON, c.RealNow())
	return err
}

func pointerToTime(value time.Time) *time.Time {
	normalized := normalizeJakartaTime(value)
	return &normalized
}

func pointerToInt(value int) *int {
	return &value
}

func CalculateEffectiveSystemTime(overrideBaseTime time.Time, overrideStartedRealAt time.Time, realNow time.Time) time.Time {
	return normalizeJakartaTime(overrideBaseTime.Add(realNow.Sub(overrideStartedRealAt)))
}
