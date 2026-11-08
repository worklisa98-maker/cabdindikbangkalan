package services

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/ptchoices"
)

type ptChoiceRuleQuerier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

type PTChoiceRule struct {
	ID         int64    `json:"id"`
	TahunLulus *int     `json:"tahun_lulus,omitempty"`
	Jalur      string   `json:"jalur"`
	MaxChoices int      `json:"max_choices"`
	IsActive   bool     `json:"is_active"`
	CreatedAt  *string  `json:"created_at,omitempty"`
	UpdatedAt  *string  `json:"updated_at,omitempty"`
}

type PTChoiceRuleListResponse struct {
	SystemMaxChoices int            `json:"system_max_choices"`
	Rules            []PTChoiceRule `json:"rules"`
}

type PTChoiceRuleMapResponse struct {
	SystemMaxChoices int            `json:"system_max_choices"`
	TahunLulus       int            `json:"tahun_lulus,omitempty"`
	Rules            map[string]int `json:"rules"`
}

type PTChoiceRuleCreateInput struct {
	TahunLulus *int
	Jalur      string
	MaxChoices int
	IsActive   *bool
}

type PTChoiceRuleUpdateInput struct {
	TahunLulus *int
	Jalur      *string
	MaxChoices *int
	IsActive   *bool
}

func formatPTChoiceRuleTimestamp(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}

func normalizePTChoiceRuleJalur(raw string) (string, error) {
	jalur, ok := ptchoices.NormalizePTJalur(raw)
	if !ok {
		return "", errors.New("PT_CHOICE_RULE_JALUR_INVALID")
	}
	return jalur, nil
}

func normalizePTChoiceRuleMaxChoices(raw int) (int, error) {
	if raw < 1 || raw > ptchoices.SYSTEM_MAX_PT_CHOICES {
		return 0, errors.New("PT_CHOICE_RULE_MAX_CHOICES_INVALID")
	}
	return raw, nil
}

func scanPTChoiceRule(scanner interface {
	Scan(dest ...any) error
}) (PTChoiceRule, error) {
	var (
		item      PTChoiceRule
		createdAt *time.Time
		updatedAt *time.Time
	)
	if err := scanner.Scan(&item.ID, &item.TahunLulus, &item.Jalur, &item.MaxChoices, &item.IsActive, &createdAt, &updatedAt); err != nil {
		return PTChoiceRule{}, err
	}
	item.CreatedAt = formatPTChoiceRuleTimestamp(createdAt)
	item.UpdatedAt = formatPTChoiceRuleTimestamp(updatedAt)
	return item, nil
}

func isPTChoiceRulesTableMissing(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42P01"
}

func getPTChoiceLimitFromQuerier(ctx context.Context, q ptChoiceRuleQuerier, tahunLulus int, jalur string) (int, error) {
	normalizedJalur, ok := ptchoices.NormalizePTJalur(jalur)
	if !ok {
		return ptchoices.SYSTEM_MAX_PT_CHOICES, newUserFacingValidationError("Jalur PT tidak valid.")
	}

	var maxChoices int
	err := q.QueryRow(ctx, `
		SELECT max_choices
		FROM pt_choice_rules
		WHERE is_active = TRUE
		  AND jalur = $1
		  AND (tahun_lulus = $2 OR tahun_lulus IS NULL)
		ORDER BY CASE WHEN tahun_lulus = $2 THEN 0 ELSE 1 END,
		         tahun_lulus DESC NULLS LAST,
		         id DESC
		LIMIT 1
	`, normalizedJalur, tahunLulus).Scan(&maxChoices)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isPTChoiceRulesTableMissing(err) {
			return ptchoices.SYSTEM_MAX_PT_CHOICES, nil
		}
		return 0, err
	}

	return ptchoices.ClampMaxChoices(maxChoices), nil
}

func GetPTChoiceLimit(ctx context.Context, q ptChoiceRuleQuerier, tahunLulus int, jalur string) int {
	limit, err := getPTChoiceLimitFromQuerier(ctx, q, tahunLulus, jalur)
	if err != nil {
		return ptchoices.SYSTEM_MAX_PT_CHOICES
	}
	return ptchoices.ClampMaxChoices(limit)
}

func ValidatePTChoiceCount(ctx context.Context, q ptChoiceRuleQuerier, tahunLulus int, jalur string, count int) error {
	limit, err := getPTChoiceLimitFromQuerier(ctx, q, tahunLulus, jalur)
	if err != nil {
		return err
	}
	if count <= limit {
		return nil
	}
	return newUserFacingValidationError(buildPTChoiceLimitMessage(jalur, tahunLulus, limit, ptchoices.DisallowedChoiceSlots(limit)))
}

func ValidatePTChoiceSlot(ctx context.Context, q ptChoiceRuleQuerier, tahunLulus int, jalur string, pilihanKe int) error {
	if !ptchoices.IsValidChoiceSlot(pilihanKe) {
		return newUserFacingValidationError(
			fmt.Sprintf(
				"Pilihan wajib diisi 1, 2, 3, atau %d.",
				ptchoices.SYSTEM_MAX_PT_CHOICES,
			),
		)
	}

	limit, err := getPTChoiceLimitFromQuerier(ctx, q, tahunLulus, jalur)
	if err != nil {
		return err
	}
	if pilihanKe <= limit {
		return nil
	}

	normalizedJalur, _ := ptchoices.NormalizePTJalur(jalur)
	return newUserFacingValidationError(
		fmt.Sprintf(
			"Pilihan ke-%d tidak diperbolehkan untuk jalur %s tahun lulus %d karena batas aktif adalah %d.",
			pilihanKe,
			normalizedJalur,
			tahunLulus,
			limit,
		),
	)
}

func ValidatePTChoiceSlots(ctx context.Context, q ptChoiceRuleQuerier, tahunLulus int, jalur string, slots []int) error {
	if len(slots) == 0 {
		return nil
	}

	limit, err := getPTChoiceLimitFromQuerier(ctx, q, tahunLulus, jalur)
	if err != nil {
		return err
	}

	invalidSlots := make([]int, 0, len(slots))
	for _, slot := range slots {
		if !ptchoices.IsValidChoiceSlot(slot) {
			return newUserFacingValidationError(
				fmt.Sprintf(
					"Pilihan wajib diisi 1, 2, 3, atau %d ketika hasil pengumuman adalah Diterima.",
					ptchoices.SYSTEM_MAX_PT_CHOICES,
				),
			)
		}
		if slot > limit {
			invalidSlots = append(invalidSlots, slot)
		}
	}
	if len(invalidSlots) == 0 {
		return nil
	}
	sort.Ints(invalidSlots)
	return newUserFacingValidationError(buildPTChoiceLimitMessage(jalur, tahunLulus, limit, invalidSlots))
}

func GetPTChoiceLimitMap(ctx context.Context, q ptChoiceRuleQuerier, tahunLulus int) (map[string]int, error) {
	result := make(map[string]int, len(ptchoices.JalurOrder))
	for _, jalur := range ptchoices.JalurOrder {
		limit, err := getPTChoiceLimitFromQuerier(ctx, q, tahunLulus, jalur)
		if err != nil {
			return nil, err
		}
		result[jalur] = limit
	}
	return result, nil
}

func GetPTChoiceMinLimit(ctx context.Context, q ptChoiceRuleQuerier, tahunLulus int, jalurs []string) (int, error) {
	if len(jalurs) == 0 {
		return ptchoices.SYSTEM_MAX_PT_CHOICES, nil
	}

	minLimit := ptchoices.SYSTEM_MAX_PT_CHOICES
	for _, rawJalur := range jalurs {
		limit, err := getPTChoiceLimitFromQuerier(ctx, q, tahunLulus, rawJalur)
		if err != nil {
			return 0, err
		}
		if limit < minLimit {
			minLimit = limit
		}
	}
	return minLimit, nil
}

func buildPTChoiceLimitMessage(jalur string, tahunLulus int, limit int, invalidSlots []int) string {
	normalizedJalur, ok := ptchoices.NormalizePTJalur(jalur)
	if !ok {
		normalizedJalur = strings.ToUpper(strings.TrimSpace(jalur))
	}

	slotMessage := ptchoices.FormatChoiceSlots(invalidSlots)
	if slotMessage == "" {
		slotMessage = ptchoices.FormatChoiceSlots(ptchoices.DisallowedChoiceSlots(limit))
	}

	return fmt.Sprintf(
		"Jalur %s tahun lulus %d maksimal %d pilihan. %s tidak boleh diisi.",
		normalizedJalur,
		tahunLulus,
		limit,
		slotMessage,
	)
}

func (s AdminService) ListPTChoiceRules(ctx context.Context) (PTChoiceRuleListResponse, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		SELECT id,
		       tahun_lulus,
		       jalur,
		       max_choices,
		       is_active,
		       created_at,
		       updated_at
		FROM pt_choice_rules
		ORDER BY tahun_lulus ASC NULLS FIRST, jalur ASC, id ASC
	`)
	if err != nil {
		return PTChoiceRuleListResponse{}, err
	}
	defer rows.Close()

	items := make([]PTChoiceRule, 0)
	for rows.Next() {
		item, scanErr := scanPTChoiceRule(rows)
		if scanErr != nil {
			return PTChoiceRuleListResponse{}, scanErr
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PTChoiceRuleListResponse{}, err
	}

	return PTChoiceRuleListResponse{
		SystemMaxChoices: ptchoices.SYSTEM_MAX_PT_CHOICES,
		Rules:            items,
	}, nil
}

func (s AdminService) CreatePTChoiceRule(ctx context.Context, input PTChoiceRuleCreateInput) (PTChoiceRule, error) {
	jalur, err := normalizePTChoiceRuleJalur(input.Jalur)
	if err != nil {
		return PTChoiceRule{}, err
	}
	maxChoices, err := normalizePTChoiceRuleMaxChoices(input.MaxChoices)
	if err != nil {
		return PTChoiceRule{}, err
	}
	if input.TahunLulus != nil && *input.TahunLulus <= 0 {
		return PTChoiceRule{}, errors.New("PT_CHOICE_RULE_TAHUN_LULUS_INVALID")
	}

	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	row := s.DB.Pool.QueryRow(ctx, `
		INSERT INTO pt_choice_rules (tahun_lulus, jalur, max_choices, is_active, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		RETURNING id,
		          tahun_lulus,
		          jalur,
		          max_choices,
		          is_active,
		          created_at,
		          updated_at
	`, input.TahunLulus, jalur, maxChoices, isActive)
	item, scanErr := scanPTChoiceRule(row)
	if scanErr != nil {
		if isPTChoiceRuleUniqueViolation(scanErr) {
			return PTChoiceRule{}, errors.New("PT_CHOICE_RULE_DUPLICATE")
		}
		return PTChoiceRule{}, scanErr
	}
	return item, nil
}

func (s AdminService) UpdatePTChoiceRule(ctx context.Context, id int64, input PTChoiceRuleUpdateInput) (PTChoiceRule, error) {
	if id <= 0 {
		return PTChoiceRule{}, errors.New("PT_CHOICE_RULE_ID_INVALID")
	}

	var (
		tahunLulus *int
		jalur      *string
		maxChoices *int
	)
	if input.TahunLulus != nil {
		if *input.TahunLulus <= 0 {
			return PTChoiceRule{}, errors.New("PT_CHOICE_RULE_TAHUN_LULUS_INVALID")
		}
		tahunLulus = input.TahunLulus
	}
	if input.Jalur != nil {
		normalizedJalur, err := normalizePTChoiceRuleJalur(*input.Jalur)
		if err != nil {
			return PTChoiceRule{}, err
		}
		jalur = &normalizedJalur
	}
	if input.MaxChoices != nil {
		normalizedMaxChoices, err := normalizePTChoiceRuleMaxChoices(*input.MaxChoices)
		if err != nil {
			return PTChoiceRule{}, err
		}
		maxChoices = &normalizedMaxChoices
	}
	if tahunLulus == nil && jalur == nil && maxChoices == nil && input.IsActive == nil {
		return PTChoiceRule{}, errors.New("INVALID_PAYLOAD")
	}

	row := s.DB.Pool.QueryRow(ctx, `
		UPDATE pt_choice_rules
		SET tahun_lulus = COALESCE($2, tahun_lulus),
		    jalur = COALESCE($3, jalur),
		    max_choices = COALESCE($4, max_choices),
		    is_active = COALESCE($5, is_active),
		    updated_at = NOW()
		WHERE id = $1
		RETURNING id,
		          tahun_lulus,
		          jalur,
		          max_choices,
		          is_active,
		          created_at,
		          updated_at
	`, id, tahunLulus, jalur, maxChoices, input.IsActive)
	item, err := scanPTChoiceRule(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PTChoiceRule{}, errors.New("PT_CHOICE_RULE_NOT_FOUND")
		}
		if isPTChoiceRuleUniqueViolation(err) {
			return PTChoiceRule{}, errors.New("PT_CHOICE_RULE_DUPLICATE")
		}
		return PTChoiceRule{}, err
	}
	return item, nil
}

func isPTChoiceRuleUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s SiswaService) GetPTChoiceRuleMap(ctx context.Context, tahunLulus int) (PTChoiceRuleMapResponse, error) {
	rules, err := GetPTChoiceLimitMap(ctx, s.DB.Pool, tahunLulus)
	if err != nil {
		return PTChoiceRuleMapResponse{}, err
	}
	return PTChoiceRuleMapResponse{
		SystemMaxChoices: ptchoices.SYSTEM_MAX_PT_CHOICES,
		TahunLulus:       tahunLulus,
		Rules:            rules,
	}, nil
}
