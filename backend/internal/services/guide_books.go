package services

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/microcosm-cc/bluemonday"
	"golang.org/x/net/html"

	"cabdin-bangkalan-backend/internal/db"
	"cabdin-bangkalan-backend/internal/logging"
)

const (
	guideBookLogModule             = "guide_books"
	GuideBookRoleAll               = "ALL"
	GuideBookRoleSchoolAdmin       = "SCHOOL_ADMIN"
	GuideBookRoleGaspolSchoolUser  = "USER_SEKOLAH_GASPOL"
	GuideBookRoleAdmin             = "ADMIN"
	GuideBookRoleAdminCabdin       = "ADMIN_CABDIN"
	GuideBookRoleAdminMaster       = "ADMIN_MASTER"
	GuideBookRolePengawas          = "PENGAWAS"
	GuideBookRoleValidator         = "VALIDATOR"
	GuideBookPlacementKartiniLomba = "LOMBA_POSTER_KARTINI"

	GuideBookLayoutTextLeftImageRight  = "TEXT_LEFT_IMAGE_RIGHT"
	GuideBookLayoutImageLeftTextRight  = "IMAGE_LEFT_TEXT_RIGHT"
	GuideBookLayoutImageLeftImageRight = "IMAGE_LEFT_IMAGE_RIGHT"
	GuideBookLayoutFullText            = "FULL_TEXT"
	GuideBookLayoutFullImage           = "FULL_IMAGE"
	GuideBookLayoutTextTopTwoImages    = "TEXT_TOP_TWO_IMAGES"

	GuideBookColumnRatioEqual     = "1:1"
	GuideBookColumnRatioTextWide  = "3:2"
	GuideBookColumnRatioImageWide = "2:3"

	maxGuideBookSlides = 10
)

var (
	ErrGuideBookNotFound   = errors.New("guide_book_not_found")
	ErrGuideBookValidation = errors.New("guide_book_validation")
	ErrGuideBookConflict   = errors.New("guide_book_conflict")

	guideBookAllowedRoles = map[string]struct{}{
		GuideBookRoleAll:              {},
		GuideBookRoleSchoolAdmin:      {},
		GuideBookRoleGaspolSchoolUser: {},
		GuideBookRoleAdmin:            {},
		GuideBookRoleAdminCabdin:      {},
		GuideBookRoleAdminMaster:      {},
		GuideBookRolePengawas:         {},
		GuideBookRoleValidator:        {},
	}

	guideBookAllowedPlacementKeys = map[string]struct{}{
		GuideBookPlacementKartiniLomba: {},
	}

	guideBookAllowedLayoutTypes = map[string]struct{}{
		GuideBookLayoutTextLeftImageRight:  {},
		GuideBookLayoutImageLeftTextRight:  {},
		GuideBookLayoutImageLeftImageRight: {},
		GuideBookLayoutFullText:            {},
		GuideBookLayoutFullImage:           {},
		GuideBookLayoutTextTopTwoImages:    {},
	}

	guideBookAllowedColumnRatios = map[string]struct{}{
		GuideBookColumnRatioEqual:     {},
		GuideBookColumnRatioTextWide:  {},
		GuideBookColumnRatioImageWide: {},
	}

	guideBookImageSrcAllowedPattern = regexp.MustCompile(`^(?:/api/(?:public/)?guide-books/images/[a-zA-Z0-9._-]+|https://[^\s"'<>]+)$`)
	guideBookTextAlignStylePattern  = regexp.MustCompile(`(?i)^\s*text-align\s*:\s*(left|center|right|justify)\s*;?\s*$`)
)

type GuideBookService struct {
	DB *db.DB
}

type GuideBook struct {
	ID           int64            `json:"id"`
	Role         string           `json:"role"`
	Title        string           `json:"title"`
	ContentHTML  string           `json:"content_html"`
	SortOrder    int              `json:"sort_order"`
	IsActive     bool             `json:"is_active"`
	PlacementKey string           `json:"placement_key"`
	IsPublic     bool             `json:"is_public"`
	Slides       []GuideBookSlide `json:"slides"`
	CreatedAt    time.Time        `json:"created_at"`
	UpdatedAt    time.Time        `json:"updated_at"`
}

type GuideBookSlide struct {
	ID          int64     `json:"id"`
	LayoutType  string    `json:"layout_type"`
	TextHTML    string    `json:"text_html"`
	Image1URL   string    `json:"image_1_url"`
	Image2URL   string    `json:"image_2_url"`
	ColumnRatio string    `json:"column_ratio"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type GuideBookUpsertInput struct {
	Role         string
	Title        string
	ContentHTML  string
	SortOrder    int
	IsActive     bool
	PlacementKey string
	IsPublic     bool
	Slides       []GuideBookSlideUpsertInput
}

type GuideBookSlideUpsertInput struct {
	LayoutType  string
	TextHTML    string
	Image1URL   string
	Image2URL   string
	ColumnRatio string
	SortOrder   int
}

type GuideBookAdminFilters struct {
	Role            string
	IncludeInactive bool
}

type GuideBookMediaInput struct {
	FileName string
	MIME     string
	Size     int
}

type guideBookScanner interface {
	Scan(dest ...any) error
}

func NormalizeGuideBookRole(role string) string {
	return strings.ToUpper(strings.TrimSpace(role))
}

func NormalizeGuideBookPlacementKey(placementKey string) string {
	return strings.ToUpper(strings.TrimSpace(placementKey))
}

func NormalizeGuideBookLayoutType(layoutType string) string {
	return strings.ToUpper(strings.TrimSpace(layoutType))
}

func NormalizeGuideBookColumnRatio(columnRatio string) string {
	normalized := strings.TrimSpace(columnRatio)
	if normalized == "" {
		return GuideBookColumnRatioEqual
	}
	return normalized
}

func IsGuideBookViewerRole(role string) bool {
	return ValidateGuideBookRole(role, false)
}

func ValidateGuideBookRole(role string, allowAll bool) bool {
	normalized := NormalizeGuideBookRole(role)
	if !allowAll && normalized == GuideBookRoleAll {
		return false
	}
	_, ok := guideBookAllowedRoles[normalized]
	return ok
}

func ValidateGuideBookPlacementKey(placementKey string) bool {
	normalized := NormalizeGuideBookPlacementKey(placementKey)
	if normalized == "" {
		return true
	}
	_, ok := guideBookAllowedPlacementKeys[normalized]
	return ok
}

func GuideBookEffectiveRole(userRole string) (string, bool) {
	normalized := NormalizeGuideBookRole(userRole)
	switch normalized {
	case GuideBookRoleSchoolAdmin,
		GuideBookRoleGaspolSchoolUser,
		GuideBookRoleAdmin,
		GuideBookRoleAdminCabdin,
		GuideBookRoleAdminMaster,
		GuideBookRolePengawas,
		GuideBookRoleValidator:
		return normalized, true
	default:
		return "", false
	}
}

func (s GuideBookService) ListVisible(ctx context.Context, effectiveRole string) ([]GuideBook, error) {
	if !ValidateGuideBookRole(effectiveRole, false) {
		return nil, ErrGuideBookValidation
	}
	ok, missing, err := tablesExist(ctx, s.DB, "guide_books")
	if err != nil {
		logging.Errorf(guideBookLogModule, "guide_books.list_visible", "stage=tables_exist role=%q err=%v", NormalizeGuideBookRole(effectiveRole), err)
		return nil, err
	}
	if !ok {
		logging.Warnf(guideBookLogModule, "guide_books.list_visible", "stage=tables_exist role=%q missing_tables=%v", NormalizeGuideBookRole(effectiveRole), missing)
		return []GuideBook{}, nil
	}

	rows, err := s.DB.Pool.Query(ctx, `
		SELECT id, role, title, content_html, sort_order, is_active, placement_key, is_public, created_at, updated_at
		FROM guide_books
		WHERE is_active = TRUE
		  AND role IN ('ALL', $1)
		ORDER BY
		  CASE WHEN role = 'ALL' THEN 0 ELSE 1 END,
		  sort_order ASC,
		  updated_at DESC,
		  id DESC
	`, NormalizeGuideBookRole(effectiveRole))
	if err != nil {
		logging.Errorf(guideBookLogModule, "guide_books.list_visible", "stage=query role=%q err=%v", NormalizeGuideBookRole(effectiveRole), err)
		return nil, err
	}
	defer rows.Close()

	items := make([]GuideBook, 0)
	for rows.Next() {
		var item GuideBook
		if err := scanGuideBookRecord(rows, &item); err != nil {
			logging.Errorf(guideBookLogModule, "guide_books.list_visible", "stage=scan role=%q items_loaded=%d err=%v", NormalizeGuideBookRole(effectiveRole), len(items), err)
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		logging.Errorf(guideBookLogModule, "guide_books.list_visible", "stage=rows_err role=%q items_loaded=%d err=%v", NormalizeGuideBookRole(effectiveRole), len(items), err)
		return nil, err
	}
	logging.Debugf(guideBookLogModule, "guide_books.list_visible", "stage=attach_slides role=%q items_loaded=%d", NormalizeGuideBookRole(effectiveRole), len(items))
	items, err = s.attachSlides(ctx, items)
	if err != nil {
		logging.Errorf(guideBookLogModule, "guide_books.list_visible", "stage=attach_slides_fallback role=%q items_loaded=%d err=%v", NormalizeGuideBookRole(effectiveRole), len(items), err)
		for index := range items {
			items[index].Slides = []GuideBookSlide{}
		}
		return items, nil
	}
	return items, nil
}

func (s GuideBookService) ListAdmin(ctx context.Context, filters GuideBookAdminFilters) ([]GuideBook, error) {
	ok, _, err := tablesExist(ctx, s.DB, "guide_books")
	if err != nil {
		return nil, err
	}
	if !ok {
		return []GuideBook{}, nil
	}

	args := make([]any, 0, 1)
	clauses := make([]string, 0, 2)
	if !filters.IncludeInactive {
		clauses = append(clauses, "is_active = TRUE")
	}
	if strings.TrimSpace(filters.Role) != "" {
		role := NormalizeGuideBookRole(filters.Role)
		if !ValidateGuideBookRole(role, true) {
			return nil, ErrGuideBookValidation
		}
		args = append(args, role)
		clauses = append(clauses, "role = $1")
	}

	query := `
		SELECT id, role, title, content_html, sort_order, is_active, placement_key, is_public, created_at, updated_at
		FROM guide_books
	`
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += `
		ORDER BY
		  CASE role
		    WHEN 'ALL' THEN 0
		    WHEN 'SCHOOL_ADMIN' THEN 1
		    WHEN 'USER_SEKOLAH_GASPOL' THEN 2
		    WHEN 'ADMIN' THEN 3
		    WHEN 'ADMIN_CABDIN' THEN 4
		    WHEN 'ADMIN_MASTER' THEN 5
		    WHEN 'PENGAWAS' THEN 6
		    WHEN 'VALIDATOR' THEN 7
		    ELSE 99
		  END,
		  sort_order ASC,
		  updated_at DESC,
		  id DESC
	`

	rows, err := s.DB.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]GuideBook, 0)
	for rows.Next() {
		var item GuideBook
		if err := scanGuideBookRecord(rows, &item); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return s.attachSlides(ctx, items)
}

func (s GuideBookService) Create(ctx context.Context, input GuideBookUpsertInput) (GuideBook, error) {
	validated, err := validateGuideBookUpsertInput(input)
	if err != nil {
		return GuideBook{}, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return GuideBook{}, err
	}
	defer tx.Rollback(ctx)

	var id int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO guide_books (
			role,
			title,
			content_html,
			sort_order,
			is_active,
			placement_key,
			is_public
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
		RETURNING id
	`,
		validated.Role,
		validated.Title,
		validated.ContentHTML,
		validated.SortOrder,
		validated.IsActive,
		validated.PlacementKey,
		validated.IsPublic,
	).Scan(&id); err != nil {
		return GuideBook{}, mapGuideBookWriteError(err)
	}

	if err := s.replaceSlides(ctx, tx, id, validated.Slides); err != nil {
		return GuideBook{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return GuideBook{}, mapGuideBookWriteError(err)
	}

	item, err := s.GetByID(ctx, id)
	if err != nil {
		return GuideBook{}, err
	}
	return item, nil
}

func (s GuideBookService) Update(ctx context.Context, id int64, input GuideBookUpsertInput) (GuideBook, error) {
	if id <= 0 {
		return GuideBook{}, ErrGuideBookValidation
	}
	validated, err := validateGuideBookUpsertInput(input)
	if err != nil {
		return GuideBook{}, err
	}

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return GuideBook{}, err
	}
	defer tx.Rollback(ctx)

	tag, err := tx.Exec(ctx, `
		UPDATE guide_books
		SET role = $2,
		    title = $3,
		    content_html = $4,
		    sort_order = $5,
		    is_active = $6,
		    placement_key = NULLIF($7, ''),
		    is_public = $8,
		    updated_at = NOW()
		WHERE id = $1
	`, id,
		validated.Role,
		validated.Title,
		validated.ContentHTML,
		validated.SortOrder,
		validated.IsActive,
		validated.PlacementKey,
		validated.IsPublic,
	)
	if err != nil {
		return GuideBook{}, mapGuideBookWriteError(err)
	}
	if tag.RowsAffected() == 0 {
		return GuideBook{}, ErrGuideBookNotFound
	}

	if err := s.replaceSlides(ctx, tx, id, validated.Slides); err != nil {
		return GuideBook{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return GuideBook{}, mapGuideBookWriteError(err)
	}

	item, err := s.GetByID(ctx, id)
	if err != nil {
		return GuideBook{}, err
	}
	return item, nil
}

func (s GuideBookService) GetByID(ctx context.Context, id int64) (GuideBook, error) {
	if id <= 0 {
		return GuideBook{}, ErrGuideBookValidation
	}
	ok, _, err := tablesExist(ctx, s.DB, "guide_books")
	if err != nil {
		return GuideBook{}, err
	}
	if !ok {
		return GuideBook{}, ErrGuideBookNotFound
	}

	var item GuideBook
	err = scanGuideBookRecord(s.DB.Pool.QueryRow(ctx, `
		SELECT id, role, title, content_html, sort_order, is_active, placement_key, is_public, created_at, updated_at
		FROM guide_books
		WHERE id = $1
	`, id), &item)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return GuideBook{}, ErrGuideBookNotFound
		}
		return GuideBook{}, err
	}
	items, err := s.attachSlides(ctx, []GuideBook{item})
	if err != nil {
		return GuideBook{}, err
	}
	return items[0], nil
}

func (s GuideBookService) GetActiveByPlacement(ctx context.Context, placementKey string, publicOnly bool) (*GuideBook, error) {
	normalizedPlacement := NormalizeGuideBookPlacementKey(placementKey)
	if normalizedPlacement == "" || !ValidateGuideBookPlacementKey(normalizedPlacement) {
		return nil, ErrGuideBookValidation
	}
	ok, _, err := tablesExist(ctx, s.DB, "guide_books")
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}

	query := `
		SELECT id, role, title, content_html, sort_order, is_active, placement_key, is_public, created_at, updated_at
		FROM guide_books
		WHERE is_active = TRUE
		  AND placement_key = $1
	`
	args := []any{normalizedPlacement}
	if publicOnly {
		query += " AND is_public = TRUE"
	}
	query += `
		ORDER BY updated_at DESC, id DESC
		LIMIT 1
	`

	var item GuideBook
	err = scanGuideBookRecord(s.DB.Pool.QueryRow(ctx, query, args...), &item)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	items, err := s.attachSlides(ctx, []GuideBook{item})
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

func (s GuideBookService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrGuideBookValidation
	}
	tag, err := s.DB.Pool.Exec(ctx, `DELETE FROM guide_books WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrGuideBookNotFound
	}
	return nil
}

func (s GuideBookService) RecordMedia(ctx context.Context, input GuideBookMediaInput) error {
	ok, _, err := tablesExist(ctx, s.DB, "guide_book_media")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	if strings.TrimSpace(input.FileName) == "" || strings.TrimSpace(input.MIME) == "" || input.Size <= 0 {
		return ErrGuideBookValidation
	}
	_, err = s.DB.Pool.Exec(ctx, `
		INSERT INTO guide_book_media (file_name, mime, size)
		VALUES ($1, $2, $3)
	`, strings.TrimSpace(input.FileName), strings.TrimSpace(input.MIME), input.Size)
	return err
}

func (s GuideBookService) CanServePublicImage(ctx context.Context, fileName string) (bool, error) {
	name := strings.TrimSpace(fileName)
	if name == "" {
		return false, ErrGuideBookValidation
	}
	ok, _, err := tablesExist(ctx, s.DB, "guide_books", "guide_book_slides")
	if err != nil {
		return false, err
	}
	if !ok {
		return false, nil
	}

	storedURL := GuideBookStoredImageURL(name)
	publicURL := GuideBookPublicImageURL(storedURL)

	var exists bool
	err = s.DB.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM guide_books gb
			INNER JOIN guide_book_slides gbs
				ON gbs.guide_book_id = gb.id
			WHERE gb.is_active = TRUE
			  AND gb.is_public = TRUE
			  AND (
				gbs.image_1_url IN ($1, $2)
				OR gbs.image_2_url IN ($1, $2)
			  )
		)
	`, storedURL, publicURL).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

func (s GuideBookService) attachSlides(ctx context.Context, items []GuideBook) ([]GuideBook, error) {
	if len(items) == 0 {
		return items, nil
	}
	slidesByGuideID, err := s.loadSlidesMap(ctx, items)
	if err != nil {
		logging.Errorf(guideBookLogModule, "guide_books.attach_slides", "guide_book_ids=%d err=%v", len(items), err)
		return items, err
	}
	for index := range items {
		items[index].Slides = slidesByGuideID[items[index].ID]
	}
	return items, nil
}

func (s GuideBookService) loadSlidesMap(ctx context.Context, items []GuideBook) (map[int64][]GuideBookSlide, error) {
	result := make(map[int64][]GuideBookSlide, len(items))
	ok, missing, err := tablesExist(ctx, s.DB, "guide_book_slides")
	if err != nil {
		logging.Errorf(guideBookLogModule, "guide_books.load_slides", "stage=tables_exist guide_book_ids=%d err=%v", len(items), err)
		return nil, err
	}
	if !ok {
		logging.Warnf(guideBookLogModule, "guide_books.load_slides", "stage=tables_exist guide_book_ids=%d missing_tables=%v", len(items), missing)
		return result, nil
	}

	ids := make([]int64, 0, len(items))
	for _, item := range items {
		result[item.ID] = []GuideBookSlide{}
		ids = append(ids, item.ID)
	}

	rows, err := s.DB.Pool.Query(ctx, `
		SELECT id, guide_book_id, layout_type, text_html, image_1_url, image_2_url, column_ratio, sort_order, created_at, updated_at
		FROM guide_book_slides
		WHERE guide_book_id = ANY($1)
		ORDER BY sort_order ASC, id ASC
	`, ids)
	if err != nil {
		logging.Errorf(guideBookLogModule, "guide_books.load_slides", "stage=query guide_book_ids=%d err=%v", len(ids), err)
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var slide GuideBookSlide
		var guideBookID int64
		if err := rows.Scan(
			&slide.ID,
			&guideBookID,
			&slide.LayoutType,
			&slide.TextHTML,
			&slide.Image1URL,
			&slide.Image2URL,
			&slide.ColumnRatio,
			&slide.SortOrder,
			&slide.CreatedAt,
			&slide.UpdatedAt,
		); err != nil {
			logging.Errorf(guideBookLogModule, "guide_books.load_slides", "stage=scan guide_book_ids=%d err=%v", len(ids), err)
			return nil, err
		}
		slide.LayoutType = NormalizeGuideBookLayoutType(slide.LayoutType)
		slide.ColumnRatio = NormalizeGuideBookColumnRatio(slide.ColumnRatio)
		slide.TextHTML = sanitizeGuideBookTextHTMLForOutput(slide.TextHTML)
		slide.Image1URL = sanitizeGuideBookImageURLForOutput(slide.Image1URL)
		slide.Image2URL = sanitizeGuideBookImageURLForOutput(slide.Image2URL)
		result[guideBookID] = append(result[guideBookID], slide)
	}
	if err := rows.Err(); err != nil {
		logging.Errorf(guideBookLogModule, "guide_books.load_slides", "stage=rows_err guide_book_ids=%d err=%v", len(ids), err)
		return nil, err
	}

	return result, nil
}

func scanGuideBookRecord(scanner guideBookScanner, item *GuideBook) error {
	if scanner == nil {
		return errors.New("guide book scanner is nil")
	}
	if item == nil {
		return errors.New("guide book item is nil")
	}

	var placementKey sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&item.Role,
		&item.Title,
		&item.ContentHTML,
		&item.SortOrder,
		&item.IsActive,
		&placementKey,
		&item.IsPublic,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return err
	}

	item.ContentHTML = sanitizeGuideBookHTMLForOutput(item.ContentHTML)
	if placementKey.Valid {
		item.PlacementKey = NormalizeGuideBookPlacementKey(placementKey.String)
	} else {
		item.PlacementKey = ""
	}
	return nil
}

func (s GuideBookService) replaceSlides(ctx context.Context, tx pgx.Tx, guideBookID int64, slides []GuideBookSlideUpsertInput) error {
	if _, err := tx.Exec(ctx, `DELETE FROM guide_book_slides WHERE guide_book_id = $1`, guideBookID); err != nil {
		return err
	}
	for _, slide := range slides {
		if _, err := tx.Exec(ctx, `
			INSERT INTO guide_book_slides (
				guide_book_id,
				layout_type,
				text_html,
				image_1_url,
				image_2_url,
				column_ratio,
				sort_order
			)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`,
			guideBookID,
			slide.LayoutType,
			slide.TextHTML,
			slide.Image1URL,
			slide.Image2URL,
			slide.ColumnRatio,
			slide.SortOrder,
		); err != nil {
			return err
		}
	}
	return nil
}

func validateGuideBookUpsertInput(input GuideBookUpsertInput) (GuideBookUpsertInput, error) {
	role := NormalizeGuideBookRole(input.Role)
	if !ValidateGuideBookRole(role, true) {
		return GuideBookUpsertInput{}, ErrGuideBookValidation
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return GuideBookUpsertInput{}, ErrGuideBookValidation
	}

	placementKey := NormalizeGuideBookPlacementKey(input.PlacementKey)
	if !ValidateGuideBookPlacementKey(placementKey) {
		return GuideBookUpsertInput{}, ErrGuideBookValidation
	}

	slides, err := validateGuideBookSlides(input.Slides)
	if err != nil {
		return GuideBookUpsertInput{}, err
	}

	contentHTML := strings.TrimSpace(input.ContentHTML)
	if len(slides) == 0 {
		contentHTML, err = SanitizeGuideBookHTML(contentHTML)
		if err != nil {
			return GuideBookUpsertInput{}, ErrGuideBookValidation
		}
	} else if contentHTML != "" {
		contentHTML, err = sanitizeGuideBookHTML(contentHTML, true, false)
		if err != nil {
			return GuideBookUpsertInput{}, ErrGuideBookValidation
		}
	}

	if placementKey == GuideBookPlacementKartiniLomba && len(slides) == 0 {
		return GuideBookUpsertInput{}, ErrGuideBookValidation
	}

	return GuideBookUpsertInput{
		Role:         role,
		Title:        title,
		ContentHTML:  contentHTML,
		SortOrder:    input.SortOrder,
		IsActive:     input.IsActive,
		PlacementKey: placementKey,
		IsPublic:     input.IsPublic,
		Slides:       slides,
	}, nil
}

func validateGuideBookSlides(slides []GuideBookSlideUpsertInput) ([]GuideBookSlideUpsertInput, error) {
	if len(slides) == 0 {
		return []GuideBookSlideUpsertInput{}, nil
	}
	if len(slides) > maxGuideBookSlides {
		return nil, ErrGuideBookValidation
	}

	validated := make([]GuideBookSlideUpsertInput, 0, len(slides))
	for index, slide := range slides {
		layoutType := NormalizeGuideBookLayoutType(slide.LayoutType)
		if _, ok := guideBookAllowedLayoutTypes[layoutType]; !ok {
			return nil, ErrGuideBookValidation
		}

		textHTML, err := SanitizeGuideBookTextHTML(slide.TextHTML, guideBookLayoutRequiresText(layoutType))
		if err != nil {
			return nil, ErrGuideBookValidation
		}

		image1URL := strings.TrimSpace(slide.Image1URL)
		image2URL := strings.TrimSpace(slide.Image2URL)
		if !guideBookLayoutAllowsImage1(layoutType) {
			image1URL = ""
		} else if image1URL != "" && !isGuideBookAllowedImageSrc(image1URL) {
			return nil, ErrGuideBookValidation
		}
		if !guideBookLayoutAllowsImage2(layoutType) {
			image2URL = ""
		} else if image2URL != "" && !isGuideBookAllowedImageSrc(image2URL) {
			return nil, ErrGuideBookValidation
		}
		if guideBookLayoutRequiresImage1(layoutType) && image1URL == "" {
			return nil, ErrGuideBookValidation
		}
		if guideBookLayoutRequiresImage2(layoutType) && image2URL == "" {
			return nil, ErrGuideBookValidation
		}

		columnRatio := NormalizeGuideBookColumnRatio(slide.ColumnRatio)
		if !guideBookLayoutUsesColumnRatio(layoutType) {
			columnRatio = GuideBookColumnRatioEqual
		}
		if _, ok := guideBookAllowedColumnRatios[columnRatio]; !ok {
			return nil, ErrGuideBookValidation
		}

		if !guideBookLayoutAllowsText(layoutType) {
			textHTML = ""
		}

		sortOrder := slide.SortOrder
		if sortOrder == 0 {
			sortOrder = (index + 1) * 10
		}

		validated = append(validated, GuideBookSlideUpsertInput{
			LayoutType:  layoutType,
			TextHTML:    textHTML,
			Image1URL:   image1URL,
			Image2URL:   image2URL,
			ColumnRatio: columnRatio,
			SortOrder:   sortOrder,
		})
	}
	return validated, nil
}

func guideBookLayoutAllowsText(layoutType string) bool {
	switch layoutType {
	case GuideBookLayoutTextLeftImageRight,
		GuideBookLayoutImageLeftTextRight,
		GuideBookLayoutFullText,
		GuideBookLayoutTextTopTwoImages:
		return true
	default:
		return false
	}
}

func guideBookLayoutRequiresText(layoutType string) bool {
	return guideBookLayoutAllowsText(layoutType)
}

func guideBookLayoutAllowsImage1(layoutType string) bool {
	switch layoutType {
	case GuideBookLayoutTextLeftImageRight,
		GuideBookLayoutImageLeftTextRight,
		GuideBookLayoutImageLeftImageRight,
		GuideBookLayoutFullImage,
		GuideBookLayoutTextTopTwoImages:
		return true
	default:
		return false
	}
}

func guideBookLayoutAllowsImage2(layoutType string) bool {
	switch layoutType {
	case GuideBookLayoutImageLeftImageRight, GuideBookLayoutTextTopTwoImages:
		return true
	default:
		return false
	}
}

func guideBookLayoutRequiresImage1(layoutType string) bool {
	return guideBookLayoutAllowsImage1(layoutType)
}

func guideBookLayoutRequiresImage2(layoutType string) bool {
	return guideBookLayoutAllowsImage2(layoutType)
}

func guideBookLayoutUsesColumnRatio(layoutType string) bool {
	switch layoutType {
	case GuideBookLayoutTextLeftImageRight, GuideBookLayoutImageLeftTextRight:
		return true
	default:
		return false
	}
}

func SanitizeGuideBookHTML(content string) (string, error) {
	return sanitizeGuideBookHTML(content, true, true)
}

func SanitizeGuideBookTextHTML(content string, required bool) (string, error) {
	return sanitizeGuideBookHTML(content, false, required)
}

func sanitizeGuideBookHTML(content string, allowImages bool, required bool) (string, error) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		if required {
			return "", ErrGuideBookValidation
		}
		return "", nil
	}

	policy := bluemonday.NewPolicy()
	policy.AllowElements(
		"p", "br", "strong", "em", "u",
		"ul", "ol", "li",
		"h1", "h2", "h3", "h4",
		"a",
		"code", "pre",
		"blockquote",
	)
	if allowImages {
		policy.AllowElements("img")
		policy.AllowAttrs("src").Matching(guideBookImageSrcAllowedPattern).OnElements("img")
		policy.AllowAttrs("alt", "title").OnElements("img")
	}
	policy.AllowAttrs("href", "title", "target", "rel").OnElements("a")
	policy.AllowAttrs("style").Matching(guideBookTextAlignStylePattern).OnElements("p", "h1", "h2", "h3", "h4")
	policy.AllowStandardURLs()

	sanitized := strings.TrimSpace(policy.Sanitize(trimmed))
	sanitized = strings.TrimSpace(normalizeGuideBookHTMLFragment(sanitized))
	if sanitized == "" && required {
		return "", ErrGuideBookValidation
	}
	return sanitized, nil
}

func sanitizeGuideBookHTMLForOutput(content string) string {
	sanitized, err := sanitizeGuideBookHTML(content, true, false)
	if err != nil {
		return ""
	}
	return sanitized
}

func sanitizeGuideBookTextHTMLForOutput(content string) string {
	sanitized, err := sanitizeGuideBookHTML(content, false, false)
	if err != nil {
		return ""
	}
	return sanitized
}

func sanitizeGuideBookImageURLForOutput(url string) string {
	value := strings.TrimSpace(url)
	if value == "" || !isGuideBookAllowedImageSrc(value) {
		return ""
	}
	return value
}

func GuideBookStoredImageURL(fileName string) string {
	return "/api/guide-books/images/" + strings.TrimSpace(fileName)
}

func GuideBookPublicImageURL(url string) string {
	value := strings.TrimSpace(url)
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "/api/guide-books/images/") {
		return strings.Replace(value, "/api/guide-books/images/", "/api/public/guide-books/images/", 1)
	}
	return value
}

func mapGuideBookWriteError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrGuideBookConflict
	}
	return err
}

func normalizeGuideBookHTMLFragment(fragment string) string {
	if strings.TrimSpace(fragment) == "" {
		return ""
	}

	ctx := &html.Node{Type: html.ElementNode, Data: "div"}
	nodes, err := html.ParseFragment(strings.NewReader(fragment), ctx)
	if err != nil {
		return fragment
	}

	for _, node := range nodes {
		normalizeGuideBookNode(node)
	}

	var builder strings.Builder
	for _, node := range nodes {
		_ = html.Render(&builder, node)
	}
	return builder.String()
}

func normalizeGuideBookNode(node *html.Node) {
	if node == nil {
		return
	}

	for child := node.FirstChild; child != nil; {
		next := child.NextSibling
		normalizeGuideBookNode(child)
		child = next
	}

	if node.Type != html.ElementNode {
		return
	}

	switch node.Data {
	case "a":
		target := strings.TrimSpace(getHTMLAttr(node, "target"))
		if strings.EqualFold(target, "_blank") {
			rel := strings.TrimSpace(getHTMLAttr(node, "rel"))
			setHTMLAttr(node, "rel", ensureLinkRelTokens(rel, "noopener", "noreferrer"))
		}
	case "img":
		src := strings.TrimSpace(getHTMLAttr(node, "src"))
		if !isGuideBookAllowedImageSrc(src) {
			detachHTMLNode(node)
			return
		}
	}
}

func isGuideBookAllowedImageSrc(src string) bool {
	value := strings.TrimSpace(src)
	if value == "" {
		return false
	}
	return guideBookImageSrcAllowedPattern.MatchString(value)
}

func ensureLinkRelTokens(rel string, required ...string) string {
	seen := map[string]struct{}{}
	ordered := make([]string, 0, len(required)+2)
	for _, token := range strings.Fields(strings.ToLower(strings.TrimSpace(rel))) {
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		ordered = append(ordered, token)
	}
	for _, token := range required {
		token = strings.ToLower(strings.TrimSpace(token))
		if token == "" {
			continue
		}
		if _, ok := seen[token]; ok {
			continue
		}
		seen[token] = struct{}{}
		ordered = append(ordered, token)
	}
	return strings.Join(ordered, " ")
}

func getHTMLAttr(node *html.Node, key string) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, key) {
			return attr.Val
		}
	}
	return ""
}

func setHTMLAttr(node *html.Node, key, value string) {
	for i := range node.Attr {
		if strings.EqualFold(node.Attr[i].Key, key) {
			node.Attr[i].Val = value
			return
		}
	}
	node.Attr = append(node.Attr, html.Attribute{Key: key, Val: value})
}

func detachHTMLNode(node *html.Node) {
	if node == nil || node.Parent == nil {
		return
	}
	parent := node.Parent
	if parent.FirstChild == node {
		parent.FirstChild = node.NextSibling
	}
	if parent.LastChild == node {
		parent.LastChild = node.PrevSibling
	}
	if node.PrevSibling != nil {
		node.PrevSibling.NextSibling = node.NextSibling
	}
	if node.NextSibling != nil {
		node.NextSibling.PrevSibling = node.PrevSibling
	}
	node.Parent = nil
	node.PrevSibling = nil
	node.NextSibling = nil
}
