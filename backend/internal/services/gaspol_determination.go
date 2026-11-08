package services

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/auth"
	"github.com/jackc/pgx/v5"
)

type gaspolQuery interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type GaspolObligation struct {
	SchoolID     int      `json:"schoolId"`
	SchoolName   string   `json:"schoolName"`
	Assessor     string   `json:"assessor"`
	AssessmentID *int64   `json:"assessmentId"`
	Status       string   `json:"status"`
	FinalScore   *float64 `json:"finalScore"`
}

// Start with the official cohort, including schools without an assessor.
// Join each active assessor separately; zero is complete when FINAL.
func gaspolObligations(ctx context.Context, q gaspolQuery, year int, cohort, stage string) ([]GaspolObligation, error) {
	rows, err := q.Query(ctx, `SELECT c.school_id,d.nama_sekolah,COALESCE(u.username,'Belum ditugaskan'),sa.id,COALESCE(sa.workflow_status,'MISSING'),sa.final_score
 FROM gaspol_cohorts c JOIN gaspol_result_locks l ON l.id=c.source_lock_id AND l.status='LOCKED' AND l.year=c.year
 JOIN data_sekolah d ON d.id=c.school_id
 LEFT JOIN gaspol_stage_assignments a ON a.year=c.year AND a.school_id=c.school_id AND a.stage=$3 AND a.assignment_role=$4 AND a.is_active
 LEFT JOIN users u ON u.id=a.assessor_user_id
 LEFT JOIN gaspol_stage_assessments sa ON sa.year=a.year AND sa.stage=a.stage AND sa.school_id=a.school_id AND sa.assessor_user_id=a.assessor_user_id
 WHERE c.year=$1 AND c.cohort=$2 AND c.released_at IS NULL ORDER BY c.school_id,a.id`, year, cohort, stage, stageAssignmentRole(stage))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GaspolObligation{}
	for rows.Next() {
		var r GaspolObligation
		if err = rows.Scan(&r.SchoolID, &r.SchoolName, &r.Assessor, &r.AssessmentID, &r.Status, &r.FinalScore); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func gaspolObligationsReady(rows []GaspolObligation, schools int) bool {
	seen := map[int]bool{}
	for _, r := range rows {
		seen[r.SchoolID] = true
		if r.Status != "FINAL" || r.FinalScore == nil {
			return false
		}
	}
	return len(seen) == schools
}
func gaspolVersion(v any) string {
	sum := sha256.Sum256(mustJSON(v))
	return hex.EncodeToString(sum[:])
}

func gaspolStageDataVersion(ctx context.Context, q gaspolQuery, year int) (string, error) {
	var raw string
	err := q.QueryRow(ctx, `SELECT jsonb_build_array(
 (SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM gaspol_stage_assessments a WHERE year=$1),
 (SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM gaspol_stage_assignments a WHERE year=$1),
 (SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM gaspol_assessments a WHERE year=$1),
 (SELECT jsonb_agg(to_jsonb(a) ORDER BY school_id,month) FROM gaspol_school_submissions a WHERE year=$1),
 (SELECT jsonb_agg(to_jsonb(a) ORDER BY school_id) FROM gaspol_school_participants a WHERE year=$1),
 (SELECT jsonb_agg(to_jsonb(a) ORDER BY id) FROM gaspol_cohorts a WHERE year=$1))::text`, year).Scan(&raw)
	return gaspolVersion(raw), err
}

type GaspolFinalRow struct {
	GaspolCompetitionCandidate
	Innovation   float64             `json:"innovation"`
	Votes        int                 `json:"votes"`
	Likes        int                 `json:"likes"`
	LikeVerified bool                `json:"likeVerified"`
	Favorite     GaspolFavoriteScore `json:"favorite"`
}
type GaspolFinalPreview struct {
	Year           int                           `json:"year"`
	Rows           []GaspolFinalRow              `json:"rows"`
	Obligations    map[string][]GaspolObligation `json:"obligations"`
	Blockers       []string                      `json:"blockers"`
	Version        string                        `json:"version"`
	ApprovalID     *int64                        `json:"approvalId"`
	ApprovalStatus string                        `json:"approvalStatus"`
	SessionID      int64                         `json:"sessionId"`
	VotingStatus   string                        `json:"votingStatus"`
	Denominator    int                           `json:"denominator"`
	Locked         bool                          `json:"locked"`
}
type GaspolDeterminationInput struct {
	Year         int            `json:"year"`
	Action       string         `json:"action"`
	ExportID     int64          `json:"exportId"`
	Version      string         `json:"version"`
	BADocumentID int64          `json:"baDocumentId"`
	BANumber     string         `json:"baNumber"`
	BADate       string         `json:"baDate"`
	Reason       string         `json:"reason"`
	Winners      map[string]int `json:"winners"`
	Order        []int          `json:"order"`
}

func gaspolOfficialCandidates(ctx context.Context, q gaspolQuery, year int, kind string, source bool) ([]GaspolCompetitionCandidate, error) {
	column := "ranking_payload"
	if source {
		column = "CASE WHEN source_payload='[]'::jsonb THEN ranking_payload ELSE source_payload END"
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT `+column+` FROM gaspol_result_locks WHERE year=$1 AND lock_type=$2 AND status='LOCKED' ORDER BY revision DESC LIMIT 1`, year, kind).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return []GaspolCompetitionCandidate{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []GaspolCompetitionCandidate
	err = json.Unmarshal(raw, &out)
	return out, err
}

func gaspolFinalPreview(ctx context.Context, q gaspolQuery, year int) (GaspolFinalPreview, error) {
	p := GaspolFinalPreview{Year: year, Rows: []GaspolFinalRow{}, Blockers: []string{}, Obligations: map[string][]GaspolObligation{}}
	candidates, err := gaspolOfficialCandidates(ctx, q, year, "TOP_5", false)
	if err != nil {
		return p, err
	}
	if len(candidates) != 5 {
		p.Blockers = append(p.Blockers, "Lima finalis resmi belum ditetapkan")
	}
	for _, stage := range []string{GaspolStagePresentation, "INNOVATION"} {
		obligations, e := gaspolObligations(ctx, q, year, "TOP_5", stage)
		if e != nil {
			return p, e
		}
		p.Obligations[stage] = obligations
		if !gaspolObligationsReady(obligations, 5) {
			p.Blockers = append(p.Blockers, "Penilaian "+stage+" belum lengkap")
		}
	}
	for i := range candidates {
		c := &candidates[i]
		if err = q.QueryRow(ctx, `SELECT COALESCE(npsn_sekolah,'') FROM data_sekolah WHERE id=$1`, c.SchoolID).Scan(&c.SchoolNPSN); err != nil {
			return p, err
		}
		var presentation, innovation *float64
		err = q.QueryRow(ctx, `SELECT AVG(sa.final_score) FILTER(WHERE sa.stage='PRESENTATION'),AVG(sa.final_score) FILTER(WHERE sa.stage='INNOVATION') FROM gaspol_stage_assessments sa WHERE sa.year=$1 AND sa.school_id=$2 AND sa.workflow_status='FINAL' AND EXISTS(SELECT 1 FROM gaspol_stage_assignments a WHERE a.year=sa.year AND a.stage=sa.stage AND a.school_id=sa.school_id AND a.assessor_user_id=sa.assessor_user_id AND a.is_active AND a.assignment_role IN ('PRESENTATION_ASSESSOR','INNOVATION_ASSESSOR'))`, year, c.SchoolID).Scan(&presentation, &innovation)
		if err != nil {
			return p, err
		}
		if presentation != nil {
			c.PresentationScore = *presentation
		}
		r := GaspolFinalRow{GaspolCompetitionCandidate: *c}
		if innovation != nil {
			r.Innovation = *innovation
		}
		p.Rows = append(p.Rows, r)
	}
	ranked := RankGaspolCompetition(candidates, GaspolStagePresentation)
	byID := map[int]GaspolFinalRow{}
	for _, r := range p.Rows {
		byID[r.SchoolID] = r
	}
	p.Rows = []GaspolFinalRow{}
	for _, c := range ranked {
		r := byID[c.SchoolID]
		r.GaspolCompetitionCandidate = c
		p.Rows = append(p.Rows, r)
	}
	var approvalVersion string
	err = q.QueryRow(ctx, `SELECT id,status,source_version FROM gaspol_innovation_nominations WHERE year=$1 ORDER BY revision DESC LIMIT 1`, year).Scan(&p.ApprovalID, &p.ApprovalStatus, &approvalVersion)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, err
	}
	currentInnovation := gaspolInnovationVersion(p.Obligations["INNOVATION"])
	if p.ApprovalStatus != "APPROVED" || approvalVersion != currentInnovation {
		p.Blockers = append(p.Blockers, "Persetujuan Kasi atas nilai inovasi terbaru diperlukan")
	}
	err = q.QueryRow(ctx, `SELECT id,status,voter_denominator FROM gaspol_voting_sessions WHERE year=$1 AND managed AND status IN ('CLOSED','ESTABLISHED') ORDER BY id DESC LIMIT 1`, year).Scan(&p.SessionID, &p.VotingStatus, &p.Denominator)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return p, err
	}
	if p.VotingStatus != "CLOSED" {
		p.Blockers = append(p.Blockers, "Voting harus ditutup")
	}
	var cutoff *string
	if err = q.QueryRow(ctx, `SELECT CASE WHEN like_cutoff_at<=NOW() THEN like_cutoff_at::text END FROM gaspol_competition_configs WHERE year=$1`, year).Scan(&cutoff); err != nil {
		return p, err
	}
	if cutoff == nil {
		p.Blockers = append(p.Blockers, "Batas waktu verifikasi like belum terpenuhi")
	}
	for i := range p.Rows {
		r := &p.Rows[i]
		var likes *int
		var votingCandidate bool
		err = q.QueryRow(ctx, `SELECT (SELECT COUNT(*) FROM gaspol_internal_votes v JOIN gaspol_voting_voters vr ON vr.session_id=v.session_id AND vr.user_id=v.voter_user_id AND vr.present AND vr.is_active WHERE v.session_id=$1 AND v.school_id=$2),
  (SELECT effective_like_count FROM gaspol_favorite_content WHERE year=$3 AND school_id=$2),
  EXISTS(SELECT 1 FROM gaspol_favorite_content fc JOIN gaspol_like_snapshots sn ON sn.favorite_content_id=fc.id WHERE fc.year=$3 AND fc.school_id=$2 AND sn.like_count=fc.effective_like_count AND sn.review_status='VERIFIED' AND sn.observation_at<=$4::timestamptz),
  EXISTS(SELECT 1 FROM gaspol_voting_candidates WHERE session_id=$1 AND school_id=$2)`, p.SessionID, r.SchoolID, year, cutoff).Scan(&r.Votes, &likes, &r.LikeVerified, &votingCandidate)
		if err != nil {
			return p, err
		}
		if likes != nil {
			r.Likes = *likes
		}
		if !votingCandidate {
			p.Blockers = append(p.Blockers, "Kandidat voting belum sesuai finalis resmi: "+r.SchoolName)
		}
		if !r.LikeVerified || likes == nil {
			p.Blockers = append(p.Blockers, "Like belum diverifikasi: "+r.SchoolName)
		}
		r.Favorite = ComputeGaspolFavoriteScore(r.Votes, p.Denominator, r.Likes, 800)
	}
	if err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE year=$1 AND lock_type='AWARDING' AND status='LOCKED')`, year).Scan(&p.Locked); err != nil {
		return p, err
	}
	// Excludes names, notes and audit timestamps; binds only decision-relevant data.
	inputs := []any{}
	for _, r := range p.Rows {
		inputs = append(inputs, []any{r.SchoolID, r.MonthlyScore, r.FieldStage1Score, r.FieldStage2Score, r.PresentationScore, r.Innovation, r.Votes, r.Likes, r.LikeVerified, r.Favorite})
	}
	p.Version = gaspolVersion([]any{inputs, []string{gaspolInnovationVersion(p.Obligations[GaspolStagePresentation]), gaspolInnovationVersion(p.Obligations["INNOVATION"])}, p.ApprovalID, p.ApprovalStatus, approvalVersion, p.SessionID, p.Denominator, p.VotingStatus, cutoff})
	return p, nil
}
func gaspolInnovationVersion(rows []GaspolObligation) string {
	inputs := []any{}
	for _, r := range rows {
		inputs = append(inputs, []any{r.SchoolID, r.AssessmentID, r.Status, r.FinalScore})
	}
	return gaspolVersion(inputs)
}

// Ties affecting an award require a recorded decision. An institution can win all three.
func gaspolChooseWinner(rows []GaspolFinalRow, kind string, manual int, reason string) (GaspolFinalRow, error) {
	ordered := append([]GaspolFinalRow(nil), rows...)
	score := func(r GaspolFinalRow) float64 {
		switch kind {
		case "INNOVATIVE":
			return r.Innovation
		case "FAVORITE":
			return r.Favorite.Total
		default:
			return r.Total
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		if score(ordered[i]) != score(ordered[j]) {
			return score(ordered[i]) > score(ordered[j])
		}
		if kind == "FAVORITE" {
			return ordered[i].Favorite.InternalScore > ordered[j].Favorite.InternalScore
		}
		return false
	})
	if len(ordered) == 0 {
		return GaspolFinalRow{}, errors.New("INCOMPLETE_ASSESSMENTS")
	}
	tied := []GaspolFinalRow{}
	for _, r := range ordered {
		if score(r) == score(ordered[0]) && (kind != "FAVORITE" || r.Favorite.InternalScore == ordered[0].Favorite.InternalScore) {
			tied = append(tied, r)
		}
	}
	if len(tied) == 1 {
		return ordered[0], nil
	}
	if strings.TrimSpace(reason) != "" {
		for _, r := range tied {
			if r.SchoolID == manual {
				return r, nil
			}
		}
	}
	return GaspolFinalRow{}, errors.New("MANUAL_TIE_RESOLUTION_REQUIRED")
}

func (s GaspolService) DetermineFinal(ctx context.Context, u auth.User, in GaspolDeterminationInput) (any, error) {
	if err := s.ensureAdminMaster(u); err != nil {
		return nil, err
	}
	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	// All reads use this transaction. Block competing writes until snapshot and awards commit.
	if _, err = tx.Exec(ctx, `LOCK TABLE gaspol_result_locks,gaspol_stage_assessments,gaspol_stage_assignments,gaspol_cohorts,gaspol_innovation_nominations,gaspol_voting_sessions,gaspol_voting_voters,gaspol_voting_candidates,gaspol_internal_votes,gaspol_favorite_content,gaspol_like_snapshots,gaspol_competition_configs,gaspol_awards,gaspol_ba_documents IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, err
	}
	if in.Action == "standings" || in.Action == "standings-preview" {
		return gaspolLockStandings(ctx, tx, u, in)
	}
	p, err := gaspolFinalPreview(ctx, tx, in.Year)
	if err != nil {
		return nil, err
	}
	if in.Action == "preview" {
		return p, nil
	}
	winners := map[string]GaspolFinalRow{}
	for _, kind := range []string{"BEST", "INNOVATIVE", "FAVORITE"} {
		winner, e := gaspolChooseWinner(p.Rows, kind, in.Winners[kind], in.Reason)
		if e != nil {
			return nil, e
		}
		winners[kind] = winner
	}
	version := gaspolVersion([]any{p.Version, in.Winners, strings.TrimSpace(in.Reason)})
	payload := map[string]any{"preview": p, "winners": winners, "reason": in.Reason, "version": version}
	if in.Action == "export" {
		var id int64
		if err = tx.QueryRow(ctx, `INSERT INTO gaspol_result_exports(year,data_version,payload,exported_by) VALUES($1,$2,$3::jsonb,$4) RETURNING id`, in.Year, version, string(mustJSON(payload)), u.ID).Scan(&id); err != nil {
			return nil, err
		}
		if err = gaspolAuditTx(ctx, tx, u, in.Year, "FINAL_MATERIAL_EXPORTED", "result_export", id, nil, payload); err != nil {
			return nil, err
		}
		if err = tx.Commit(ctx); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "version": version, "payload": payload}, nil
	}
	if in.Action != "lock" {
		return nil, errors.New("INVALID_PAYLOAD")
	}
	if p.Locked {
		return nil, errors.New("RESULT_ALREADY_LOCKED")
	}
	if len(p.Blockers) > 0 {
		return nil, fmt.Errorf("INCOMPLETE_ASSESSMENTS: %s", strings.Join(p.Blockers, "; "))
	}
	var exportedVersion string
	if err = tx.QueryRow(ctx, `SELECT data_version FROM gaspol_result_exports WHERE id=$1 AND year=$2`, in.ExportID, in.Year).Scan(&exportedVersion); err != nil || exportedVersion != version || in.Version != version {
		return nil, errors.New("BA_EXPORT_STALE")
	}
	if err = gaspolBindBA(ctx, tx, in.Year, in.BADocumentID, "AWARDING", in.BANumber, in.BADate); err != nil {
		return nil, err
	}
	var id int64
	if err = tx.QueryRow(ctx, `INSERT INTO gaspol_result_locks(year,lock_type,ranking_payload,source_payload,ba_document_id,reason,tie_resolution,locked_by) VALUES($1,'AWARDING',$2::jsonb,$3::jsonb,$4,$5,$6::jsonb,$7) RETURNING id`, in.Year, string(mustJSON(p.Rows)), string(mustJSON(payload)), in.BADocumentID, in.Reason, string(mustJSON(in.Winners)), u.ID).Scan(&id); err != nil {
		return nil, err
	}
	for kind, w := range winners {
		score := w.Total
		if kind == "INNOVATIVE" {
			score = w.Innovation
		}
		if kind == "FAVORITE" {
			score = w.Favorite.Total
		}
		if _, err = tx.Exec(ctx, `INSERT INTO gaspol_awards(year,award_type,school_id,score,metadata,locked_by) VALUES($1,$2,$3,$4,$5::jsonb,$6)`, in.Year, kind, w.SchoolID, score, string(mustJSON(map[string]any{"lockId": id, "baDocumentId": in.BADocumentID, "version": version, "winner": w})), u.ID); err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE gaspol_voting_sessions SET status='ESTABLISHED' WHERE id=$1`, p.SessionID); err != nil {
		return nil, err
	}
	if _, err = tx.Exec(ctx, `UPDATE gaspol_favorite_content SET status='FROZEN',frozen_at=NOW(),link_locked_at=COALESCE(link_locked_at,NOW()) WHERE year=$1`, in.Year); err != nil {
		return nil, err
	}
	if err = gaspolAuditTx(ctx, tx, u, in.Year, "AWARDING_LOCKED", "result_lock", id, nil, payload); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return payload, nil
}

func gaspolBindBA(ctx context.Context, tx pgx.Tx, year int, id int64, kind, number, date string) error {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return errors.New("INVALID_DATE")
	}
	if id <= 0 || strings.TrimSpace(number) == "" || date == "" {
		return errors.New("BA_REQUIRED")
	}
	tag, err := tx.Exec(ctx, `UPDATE gaspol_ba_documents SET document_type=$3,ba_number=$4,ba_date=$5::date WHERE id=$1 AND year=$2 AND content_type='application/pdf' AND substring(content from 1 for 5)=decode('255044462d','hex') AND NOT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE ba_document_id=$1 AND status='LOCKED')`, id, year, kind, number, date)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("BA_REQUIRED")
	}
	return nil
}

func (s GaspolService) OfficialDocumentsZIP(ctx context.Context, u auth.User, year int) ([]byte, error) {
	if !auth.IsAdminMaster(u) && auth.NormalizeRole(u.Role) != auth.RoleAdminCabdin {
		return nil, auth.ErrForbidden
	}
	rows, err := s.DB.Pool.Query(ctx, `SELECT l.lock_type,l.revision,d.file_name,d.content FROM gaspol_result_locks l JOIN gaspol_ba_documents d ON d.id=l.ba_document_id AND d.year=l.year WHERE l.year=$1 AND l.status='LOCKED' ORDER BY l.lock_type,l.revision`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	for rows.Next() {
		var kind, name string
		var revision int
		var content []byte
		if err = rows.Scan(&kind, &revision, &name, &content); err != nil {
			return nil, err
		}
		f, e := z.Create(fmt.Sprintf("%d_%s_v%d.pdf", year, kind, revision))
		if e != nil {
			return nil, e
		}
		if _, e = f.Write(content); e != nil {
			return nil, e
		}
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Monitoring exposes only the assigned innovation workflow to assessors/Kasi.
func (s GaspolService) DeterminationMonitor(ctx context.Context, u auth.User, year int) (map[string]any, error) {
	master := auth.IsAdminMaster(u)
	cabdin := auth.NormalizeRole(u.Role) == auth.RoleAdminCabdin
	var kasi, assessor bool
	err := s.DB.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='KASI_SMK_APPROVER' AND is_active),EXISTS(SELECT 1 FROM gaspol_stage_assignments WHERE year=$1 AND assessor_user_id=$2 AND assignment_role='INNOVATION_ASSESSOR' AND is_active)`, year, u.ID).Scan(&kasi, &assessor)
	if err != nil {
		return nil, err
	}
	if !master && !cabdin && !kasi && !assessor {
		return nil, auth.ErrForbidden
	}
	out := map[string]any{"master": master, "kasi": kasi, "innovationAssessor": assessor}
	p, err := gaspolFinalPreview(ctx, s.DB.Pool, year)
	if err != nil {
		return nil, err
	}
	if !master && !cabdin {
		out["innovation"] = map[string]any{"approvalId": p.ApprovalID, "approvalStatus": p.ApprovalStatus, "obligations": p.Obligations["INNOVATION"]}
		return out, nil
	}
	out["final"] = p
	approvers, err := s.DB.Pool.Query(ctx, `SELECT a.assessor_user_id,u.username FROM gaspol_stage_assignments a JOIN users u ON u.id=a.assessor_user_id WHERE a.year=$1 AND a.assignment_role='KASI_SMK_APPROVER' AND a.is_active`, year)
	if err != nil {
		return nil, err
	}
	kasiAssignments := []map[string]any{}
	for approvers.Next() {
		var id int
		var name string
		if err = approvers.Scan(&id, &name); err != nil {
			approvers.Close()
			return nil, err
		}
		kasiAssignments = append(kasiAssignments, map[string]any{"userId": id, "name": name})
	}
	err = approvers.Err()
	approvers.Close()
	if err != nil {
		return nil, err
	}
	out["kasiAssignments"] = kasiAssignments

	config, e := s.CompetitionConfig(ctx, year)
	if e != nil {
		return nil, e
	}
	out["config"] = config
	standings, e := s.officialStandings(ctx, year)
	if e != nil {
		return nil, e
	}
	out["standings"] = standings

	stages := []struct{ kind, cohort, stage string }{{"TOP_5", "TOP_10", GaspolStageField2}, {"BOTTOM_5", "NOT_FAILED", GaspolStageNomination}, {"FINAL", "BOTTOM_5", GaspolStageAppeal}}
	previews := map[string]any{}
	for _, stage := range stages {
		raw, e := s.loadCompetitionCandidates(ctx, year, stage.cohort)
		if e != nil {
			return nil, e
		}
		candidates := []GaspolCompetitionCandidate{}
		for _, r := range raw {
			candidates = append(candidates, r.GaspolCompetitionCandidate)
		}
		obligations, e := gaspolObligations(ctx, s.DB.Pool, year, stage.cohort, stage.stage)
		if e != nil {
			return nil, e
		}
		expected := map[string]int{"TOP_5": 10, "BOTTOM_5": 11, "FINAL": 5}[stage.kind]
		previews[stage.kind] = map[string]any{"rows": RankGaspolCompetition(candidates, stage.stage), "obligations": obligations, "ready": gaspolObligationsReady(obligations, expected)}
	}
	out["previews"] = previews
	rows, err := s.DB.Pool.Query(ctx, `SELECT l.id,l.lock_type,l.revision,l.ranking_payload,l.locked_at::text,l.ba_document_id,COALESCE(d.ba_number,''),COALESCE(d.ba_date::text,''),COALESCE(u.username,''),COALESCE(d.uploaded_at::text,'') FROM gaspol_result_locks l LEFT JOIN gaspol_ba_documents d ON d.id=l.ba_document_id LEFT JOIN users u ON u.id=d.uploaded_by WHERE l.year=$1 AND l.status='LOCKED' ORDER BY l.id`, year)
	if err != nil {
		return nil, err
	}
	locks := []map[string]any{}
	for rows.Next() {
		var id int64
		var kind, at, number, date, actor, uploaded string
		var rev int
		var doc *int64
		var raw []byte
		if err = rows.Scan(&id, &kind, &rev, &raw, &at, &doc, &number, &date, &actor, &uploaded); err != nil {
			rows.Close()
			return nil, err
		}
		locks = append(locks, map[string]any{"id": id, "kind": kind, "revision": rev, "rows": json.RawMessage(raw), "lockedAt": at, "documentId": doc, "baNumber": number, "baDate": date, "uploadedBy": actor, "uploadedAt": uploaded})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	out["locks"] = locks
	return out, nil
}

// Fixed zone membership derives exclusively from official determinations.
func buildGaspolZones(finalists, topTen, nomination, appeal, failed []GaspolCompetitionCandidate, order []int, reason string) ([]GaspolRankingRow, error) {
	sets := func(rows []GaspolCompetitionCandidate) map[int]bool {
		m := map[int]bool{}
		for _, r := range rows {
			m[r.SchoolID] = true
		}
		return m
	}
	finalSet, appealSet, failedSet := sets(finalists), sets(appeal), sets(failed)
	zones := [][]GaspolCompetitionCandidate{finalists, {}, {}, {}, failed}
	for _, r := range topTen {
		if !finalSet[r.SchoolID] {
			zones[1] = append(zones[1], r)
		}
	}
	for _, r := range nomination {
		if !appealSet[r.SchoolID] {
			zones[2] = append(zones[2], r)
		}
	}
	for _, r := range appeal {
		if !failedSet[r.SchoolID] {
			zones[3] = append(zones[3], r)
		}
	}
	sizes := []int{5, 5, 6, 2, 3}
	names := []string{"TOP_5", "TOP_10", "PASSED_NOMINATION", "ESCAPED_BOTTOM_ZONE", "TERGAGAL"}
	badges := []string{"Finalis", "Mantap Top 10", "Selamat dari Ter Gagal", "Selamat dari Banding", "Ter Gagal"}
	stages := []string{GaspolStagePresentation, GaspolStageField2, GaspolStageNomination, GaspolStageAppeal, GaspolStageAppeal}
	preference := map[int]int{}
	for i, id := range order {
		if _, ok := preference[id]; ok {
			return nil, errors.New("INVALID_TIE_ORDER")
		}
		preference[id] = i
	}
	seen := map[int]bool{}
	out := []GaspolRankingRow{}
	for z, zone := range zones {
		if len(zone) != sizes[z] {
			return nil, errors.New("INCOMPLETE_OFFICIAL_ZONES")
		}
		ranked := RankGaspolCompetition(zone, stages[z])
		for i := 1; i < len(ranked); i++ {
			if ranked[i].Total == ranked[i-1].Total {
				_, a := preference[ranked[i].SchoolID]
				_, b := preference[ranked[i-1].SchoolID]
				if !a || !b || strings.TrimSpace(reason) == "" {
					return nil, errors.New("MANUAL_TIE_RESOLUTION_REQUIRED")
				}
			}
		}
		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].Total == ranked[j].Total {
				return preference[ranked[i].SchoolID] < preference[ranked[j].SchoolID]
			}
			return ranked[i].Total > ranked[j].Total
		})
		for _, c := range ranked {
			if seen[c.SchoolID] {
				return nil, errors.New("DUPLICATE_OFFICIAL_SCHOOL")
			}
			seen[c.SchoolID] = true
			path := "MAIN"
			if z >= 2 {
				path = "NOT_FAILED"
			}
			out = append(out, GaspolRankingRow{Rank: len(out) + 1, Stage: stages[z], SchoolID: c.SchoolID, SchoolName: c.SchoolName, Cohort: names[z], Badge: badges[z], Path: path, MonthlyFinal: true, Scores: gaspolCompetitionScore(c, path, gaspolCandidateCompleteness{HasField1: true, HasField2: z < 2, HasPresentation: z == 0, HasNomination: z >= 2, HasAppeal: z >= 3})})
		}
	}
	return out, nil
}
func gaspolLockStandings(ctx context.Context, tx pgx.Tx, u auth.User, in GaspolDeterminationInput) (any, error) {
	var existing bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM gaspol_ranking_snapshots_v2 WHERE year=$1 AND snapshot_type='STANDINGS')`, in.Year).Scan(&existing); err != nil {
		return nil, err
	}
	if existing {
		return nil, errors.New("RESULT_ALREADY_LOCKED")
	}
	main, err := gaspolOfficialCandidates(ctx, tx, in.Year, "AWARDING", false)
	if err != nil {
		return nil, err
	}
	top, err := gaspolOfficialCandidates(ctx, tx, in.Year, "TOP_5", true)
	if err != nil {
		return nil, err
	}
	nom, err := gaspolOfficialCandidates(ctx, tx, in.Year, "BOTTOM_5", true)
	if err != nil {
		return nil, err
	}
	appeal, err := gaspolOfficialCandidates(ctx, tx, in.Year, "FINAL", true)
	if err != nil {
		return nil, err
	}
	failed, err := gaspolOfficialCandidates(ctx, tx, in.Year, "FINAL", false)
	if err != nil {
		return nil, err
	}
	rows, err := buildGaspolZones(main, top, nom, appeal, failed, in.Order, in.Reason)
	if err != nil {
		return nil, err
	}
	if in.Action == "standings-preview" {
		return map[string]any{"rows": rows, "version": gaspolVersion([]any{rows, in.Order, strings.TrimSpace(in.Reason)})}, nil
	}
	if in.Version != gaspolVersion([]any{rows, in.Order, strings.TrimSpace(in.Reason)}) {
		return nil, errors.New("RESULT_PREVIEW_STALE")
	}
	var references []byte
	if err = tx.QueryRow(ctx, `SELECT jsonb_object_agg(lock_type,id) FROM gaspol_result_locks WHERE year=$1 AND status='LOCKED'`, in.Year).Scan(&references); err != nil {
		return nil, err
	}
	for i := range rows {
		r := &rows[i]
		r.Year = in.Year
		payload := map[string]any{"row": r, "calculationVersion": "gaspol-five-zones-v1", "reason": in.Reason, "sourceReferences": json.RawMessage(references), "locked": true}
		if _, err = tx.Exec(ctx, `INSERT INTO gaspol_ranking_snapshots_v2(year,snapshot_type,school_id,rank_position,cohort,score_total,maximum_score,payload,created_by) VALUES($1,'STANDINGS',$2,$3,$4,$5,$6,$7::jsonb,$8)`, in.Year, r.SchoolID, r.Rank, r.Cohort, r.Scores.ScoreTotal, r.Scores.MaximumScore, string(mustJSON(payload)), u.ID); err != nil {
			return nil, err
		}
	}
	if err = gaspolAuditTx(ctx, tx, u, in.Year, "STANDINGS_LOCKED", "ranking_snapshot", 0, nil, in); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return rows, nil
}
func (s GaspolService) officialStandings(ctx context.Context, year int) ([]GaspolRankingRow, error) {
	rows, err := s.DB.Pool.Query(ctx, `SELECT payload->'row' FROM gaspol_ranking_snapshots_v2 WHERE year=$1 AND snapshot_type='STANDINGS' ORDER BY rank_position`, year)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []GaspolRankingRow{}
	for rows.Next() {
		var raw []byte
		var r GaspolRankingRow
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s GaspolService) publicPublishedRanking(ctx context.Context, year int, config GaspolCompetitionConfig) ([]GaspolRankingRow, string, error) {
	if !config.PublicFullRankingEnabled {
		return nil, GaspolStageMonthly, nil
	}
	rows, err := s.officialStandings(ctx, year)
	if err != nil {
		return nil, GaspolStageFinal, err
	}
	if len(rows) == 21 {
		return rows, GaspolStageFinal, nil
	}
	return s.publicLockedRanking(ctx, year, config.State)
}

func selectGaspolDetermination(rows []GaspolCompetitionCandidate, limit int, bottom bool, manual *int, order []int, reason string) ([]GaspolCompetitionCandidate, error) {
	out := append([]GaspolCompetitionCandidate(nil), rows...)
	sort.SliceStable(out, func(i, j int) bool {
		if bottom {
			return out[i].Total < out[j].Total
		}
		return out[i].Total > out[j].Total
	})
	if len(out) < limit {
		return nil, errors.New("INCOMPLETE_ASSESSMENTS")
	}
	if len(out) > limit && out[limit-1].Total == out[limit].Total {
		if strings.TrimSpace(reason) == "" {
			return nil, errors.New("MANUAL_TIE_RESOLUTION_REQUIRED")
		}
		start, end := limit-1, limit
		for start > 0 && out[start-1].Total == out[limit].Total {
			start--
		}
		for end+1 < len(out) && out[end+1].Total == out[limit].Total {
			end++
		}
		if len(order) == 0 && end-start == 1 && manual != nil {
			order = []int{*manual}
			for _, r := range out[start : end+1] {
				if r.SchoolID != *manual {
					order = append(order, r.SchoolID)
				}
			}
		}
		preference := map[int]int{}
		for i, id := range order {
			if _, ok := preference[id]; ok {
				return nil, errors.New("INVALID_TIE_ORDER")
			}
			preference[id] = i
		}
		for _, r := range out[start : end+1] {
			if _, ok := preference[r.SchoolID]; !ok {
				return nil, errors.New("MANUAL_TIE_RESOLUTION_REQUIRED")
			}
		}
		sort.SliceStable(out[start:end+1], func(i, j int) bool { return preference[out[start+i].SchoolID] < preference[out[start+j].SchoolID] })
	}
	return out[:limit], nil
}

// Only unattached uploads are disposable; historic and official documents are retained.
func (s GaspolService) DeleteStagedBA(ctx context.Context, u auth.User, id int64) error {
	if err := s.ensureAdminMaster(u); err != nil {
		return err
	}
	_, err := s.DB.Pool.Exec(ctx, `DELETE FROM gaspol_ba_documents d WHERE id=$1 AND document_type='STAGED' AND uploaded_by=$2 AND NOT EXISTS(SELECT 1 FROM gaspol_result_locks WHERE ba_document_id=d.id)`, id, u.ID)
	return err
}
