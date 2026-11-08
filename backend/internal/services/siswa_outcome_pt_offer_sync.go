package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/logging"
)

const OutcomePTOfferSourceBackfillFromFinal = "BACKFILL_FROM_FINAL"

type PTOutcomeOfferAuditSummary struct {
	TotalFinalPT                int `json:"total_final_pt"`
	TotalMissingAcceptedOffer   int `json:"total_missing_accepted_offer"`
	TotalAcceptedDetailMismatch int `json:"total_accepted_detail_mismatch"`
	TotalBackfillCandidates     int `json:"total_backfill_candidates"`
}

type PTOutcomeOfferBackfillResult struct {
	Audit                  PTOutcomeOfferAuditSummary `json:"audit"`
	Backfilled             int                        `json:"backfilled"`
	SkippedConflictingRows int                        `json:"skipped_conflicting_rows"`
	ProcessedAt            time.Time                  `json:"processed_at"`
}

type ptOutcomeOfferRecord struct {
	ID                 int64
	SchoolID           int
	TahunLulus         int
	NISN               string
	Jalur              string
	Kampus             string
	Prodi              string
	TahunDiterima      *int
	AnnouncementStatus string
	Source             string
}

type ptFinalOutcomeOfferRow struct {
	SchoolID      int
	TahunLulus    int
	NISN          string
	Jalur         string
	Kampus        string
	Prodi         string
	TahunDiterima *int
	FinalizedAt   *time.Time
}

type ptOutcomeOfferAuditDecision struct {
	FinalRow       ptFinalOutcomeOfferRow
	Reason         string
	ShouldBackfill bool
}

func shouldEnforcePTAnnouncementValidation(finalStatus string, outcomeSource string) bool {
	return strings.EqualFold(strings.TrimSpace(finalStatus), "PT") &&
		strings.EqualFold(strings.TrimSpace(outcomeSource), OutcomeSourceFinalSiswaAkhir)
}

func normalizePTOutcomeOfferJalur(value string) string {
	if normalized, ok := normalizeJalurPenerimaan(value); ok {
		return normalized
	}
	return normalizeUpper(value)
}

func normalizePTOutcomeOfferText(value string) string {
	return normalizeUpper(value)
}

func ptOfferMatchesFinalDetail(offer ptOutcomeOfferRecord, finalPath string, ptDetail OutcomePTDetail) bool {
	return normalizePTOutcomeOfferJalur(offer.Jalur) == normalizePTOutcomeOfferJalur(finalPath) &&
		normalizePTOutcomeOfferText(offer.Kampus) == normalizePTOutcomeOfferText(ptDetail.Kampus) &&
		normalizePTOutcomeOfferText(offer.Prodi) == normalizePTOutcomeOfferText(ptDetail.Prodi)
}

func classifyPTFinalizationAgainstAcceptedOffers(
	offers []ptOutcomeOfferRecord,
	finalPath string,
	ptDetail OutcomePTDetail,
) error {
	accepted := make([]ptOutcomeOfferRecord, 0, len(offers))
	for _, offer := range offers {
		if strings.EqualFold(strings.TrimSpace(offer.AnnouncementStatus), "DITERIMA") {
			accepted = append(accepted, offer)
		}
	}
	if len(accepted) == 0 {
		return ErrPTAnnouncementRequired
	}
	for _, offer := range accepted {
		if ptOfferMatchesFinalDetail(offer, finalPath, ptDetail) {
			return nil
		}
	}
	return ErrPTAnnouncementMismatch
}

func findConflictingStoredOffer(offers []ptOutcomeOfferRecord, final ptFinalOutcomeOfferRow) bool {
	target := OutcomePTDetail{
		Kampus: final.Kampus,
		Prodi:  final.Prodi,
		Jalur:  final.Jalur,
	}
	for _, offer := range offers {
		if ptOfferMatchesFinalDetail(offer, final.Jalur, target) {
			return true
		}
	}
	return false
}

func auditPTOutcomeOfferRows(
	finalRows []ptFinalOutcomeOfferRow,
	offersByKey map[string][]ptOutcomeOfferRecord,
) (PTOutcomeOfferAuditSummary, []ptOutcomeOfferAuditDecision) {
	summary := PTOutcomeOfferAuditSummary{
		TotalFinalPT: len(finalRows),
	}
	decisions := make([]ptOutcomeOfferAuditDecision, 0)

	for _, finalRow := range finalRows {
		key := buildValidationKey(finalRow.NISN, finalRow.TahunLulus)
		offers := offersByKey[key]
		err := classifyPTFinalizationAgainstAcceptedOffers(
			offers,
			finalRow.Jalur,
			OutcomePTDetail{
				Kampus:        finalRow.Kampus,
				Prodi:         finalRow.Prodi,
				Jalur:         finalRow.Jalur,
				TahunDiterima: finalRow.TahunDiterima,
			},
		)
		switch {
		case err == nil:
			continue
		case errors.Is(err, ErrPTAnnouncementRequired):
			summary.TotalMissingAcceptedOffer++
			decision := ptOutcomeOfferAuditDecision{
				FinalRow: finalRow,
				Reason:   "MISSING_ACCEPTED_OFFER",
			}
			if findConflictingStoredOffer(offers, finalRow) {
				decision.Reason = "CONFLICTING_EXISTING_OFFER"
			} else {
				decision.ShouldBackfill = true
				summary.TotalBackfillCandidates++
			}
			decisions = append(decisions, decision)
		case errors.Is(err, ErrPTAnnouncementMismatch):
			summary.TotalAcceptedDetailMismatch++
			decisions = append(decisions, ptOutcomeOfferAuditDecision{
				FinalRow: finalRow,
				Reason:   "ACCEPTED_DETAIL_MISMATCH",
			})
		}
	}

	return summary, decisions
}

func (s SiswaService) validatePTAnnouncementForOutcomeTx(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	tahunLulus int,
	nisn string,
	finalPath string,
	ptDetail *OutcomePTDetail,
) error {
	if ptDetail == nil {
		return ErrInvalidOutcome
	}
	if ok, err := txTablesExist(ctx, tx, "outcome_pt_offers"); err != nil {
		return err
	} else if !ok {
		logging.Warnf(
			"siswa_outcomes",
			"strict_pt_validation",
			"event=offer_table_missing school_id=%d tahun_lulus=%d nisn=%s",
			schoolID,
			tahunLulus,
			strings.TrimSpace(nisn),
		)
		return ErrPTAnnouncementRequired
	}

	rows, err := tx.Query(ctx, `
		SELECT id,
		       school_id,
		       tahun_lulus,
		       COALESCE(nisn, ''),
		       COALESCE(jalur, ''),
		       COALESCE(kampus, ''),
		       COALESCE(prodi, ''),
		       tahun_diterima,
		       COALESCE(announcement_status, ''),
		       COALESCE(source, '')
		FROM outcome_pt_offers
		WHERE school_id = $1
		  AND tahun_lulus = $2
		  AND nisn = $3
		  AND announcement_status = 'DITERIMA'
	`, schoolID, tahunLulus, strings.TrimSpace(nisn))
	if err != nil {
		return err
	}
	defer rows.Close()

	offers := make([]ptOutcomeOfferRecord, 0)
	for rows.Next() {
		var item ptOutcomeOfferRecord
		if err := rows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.Jalur,
			&item.Kampus,
			&item.Prodi,
			&item.TahunDiterima,
			&item.AnnouncementStatus,
			&item.Source,
		); err != nil {
			return err
		}
		offers = append(offers, item)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if err := classifyPTFinalizationAgainstAcceptedOffers(offers, finalPath, *ptDetail); err != nil {
		return err
	}
	return nil
}

func (s SiswaService) fetchPTOutcomeOfferAuditData(
	ctx context.Context,
) ([]ptFinalOutcomeOfferRow, map[string][]ptOutcomeOfferRecord, error) {
	ok, missing, err := tablesExist(ctx, s.DB, "student_outcomes", "outcome_pt_details", "outcome_pt_offers")
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return []ptFinalOutcomeOfferRow{}, map[string][]ptOutcomeOfferRecord{}, fmt.Errorf("missing PT outcome tables: %s", strings.Join(missing, ","))
	}

	finalRows, err := s.DB.Pool.Query(ctx, `
		SELECT so.school_id,
		       so.tahun_lulus,
		       COALESCE(so.nisn, ''),
		       COALESCE(so.final_path::text, ''),
		       COALESCE(pt.kampus, ''),
		       COALESCE(pt.prodi, ''),
		       pt.tahun_diterima,
		       so.finalized_at
		FROM student_outcomes so
		JOIN outcome_pt_details pt
		  ON pt.school_id = so.school_id
		 AND pt.tahun_lulus = so.tahun_lulus
		 AND pt.nisn = so.nisn
		WHERE so.final_status = 'PT'
		  AND so.finalized_at IS NOT NULL
		ORDER BY so.school_id ASC, so.tahun_lulus ASC, so.nisn ASC
	`)
	if err != nil {
		return nil, nil, err
	}
	defer finalRows.Close()

	finalResults := make([]ptFinalOutcomeOfferRow, 0)
	keys := map[string]struct{}{}
	for finalRows.Next() {
		var item ptFinalOutcomeOfferRow
		if err := finalRows.Scan(
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.Jalur,
			&item.Kampus,
			&item.Prodi,
			&item.TahunDiterima,
			&item.FinalizedAt,
		); err != nil {
			return nil, nil, err
		}
		item.NISN = strings.TrimSpace(item.NISN)
		item.Jalur = strings.TrimSpace(item.Jalur)
		item.Kampus = strings.TrimSpace(item.Kampus)
		item.Prodi = strings.TrimSpace(item.Prodi)
		finalResults = append(finalResults, item)
		keys[buildValidationKey(item.NISN, item.TahunLulus)] = struct{}{}
	}
	if err := finalRows.Err(); err != nil {
		return nil, nil, err
	}
	if len(finalResults) == 0 {
		return []ptFinalOutcomeOfferRow{}, map[string][]ptOutcomeOfferRecord{}, nil
	}

	offersRows, err := s.DB.Pool.Query(ctx, `
		WITH final_students AS (
			SELECT DISTINCT so.school_id, so.tahun_lulus, COALESCE(so.nisn, '') AS nisn
			FROM student_outcomes so
			JOIN outcome_pt_details pt
			  ON pt.school_id = so.school_id
			 AND pt.tahun_lulus = so.tahun_lulus
			 AND pt.nisn = so.nisn
			WHERE so.final_status = 'PT'
			  AND so.finalized_at IS NOT NULL
		)
		SELECT offer.id,
		       offer.school_id,
		       offer.tahun_lulus,
		       COALESCE(offer.nisn, ''),
		       COALESCE(offer.jalur, ''),
		       COALESCE(offer.kampus, ''),
		       COALESCE(offer.prodi, ''),
		       offer.tahun_diterima,
		       COALESCE(offer.announcement_status, ''),
		       COALESCE(offer.source, '')
		FROM outcome_pt_offers offer
		JOIN final_students fs
		  ON fs.school_id = offer.school_id
		 AND fs.tahun_lulus = offer.tahun_lulus
		 AND fs.nisn = offer.nisn
		ORDER BY offer.school_id ASC, offer.tahun_lulus ASC, offer.nisn ASC, offer.id ASC
	`)
	if err != nil {
		return nil, nil, err
	}
	defer offersRows.Close()

	offersByKey := make(map[string][]ptOutcomeOfferRecord, len(keys))
	for offersRows.Next() {
		var item ptOutcomeOfferRecord
		if err := offersRows.Scan(
			&item.ID,
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.Jalur,
			&item.Kampus,
			&item.Prodi,
			&item.TahunDiterima,
			&item.AnnouncementStatus,
			&item.Source,
		); err != nil {
			return nil, nil, err
		}
		key := buildValidationKey(item.NISN, item.TahunLulus)
		offersByKey[key] = append(offersByKey[key], item)
	}
	if err := offersRows.Err(); err != nil {
		return nil, nil, err
	}

	return finalResults, offersByKey, nil
}

func (s SiswaService) AuditPTOutcomeOfferBackfill(ctx context.Context) (PTOutcomeOfferAuditSummary, error) {
	finalRows, offersByKey, err := s.fetchPTOutcomeOfferAuditData(ctx)
	if err != nil {
		return PTOutcomeOfferAuditSummary{}, err
	}
	summary, decisions := auditPTOutcomeOfferRows(finalRows, offersByKey)
	logging.Infof(
		"pt_outcome_offer_backfill",
		"audit",
		"total_final_pt=%d total_missing_accepted_offer=%d total_accepted_detail_mismatch=%d total_backfill_candidates=%d",
		summary.TotalFinalPT,
		summary.TotalMissingAcceptedOffer,
		summary.TotalAcceptedDetailMismatch,
		summary.TotalBackfillCandidates,
	)
	for _, decision := range decisions {
		if decision.ShouldBackfill {
			continue
		}
		logging.Warnf(
			"pt_outcome_offer_backfill",
			"audit_detail",
			"reason=%s school_id=%d tahun_lulus=%d nisn=%s jalur=%s kampus=%s prodi=%s",
			decision.Reason,
			decision.FinalRow.SchoolID,
			decision.FinalRow.TahunLulus,
			decision.FinalRow.NISN,
			decision.FinalRow.Jalur,
			decision.FinalRow.Kampus,
			decision.FinalRow.Prodi,
		)
	}
	return summary, nil
}

func (s SiswaService) BackfillLegacyPTOutcomeOffers(ctx context.Context) (PTOutcomeOfferBackfillResult, error) {
	finalRows, offersByKey, err := s.fetchPTOutcomeOfferAuditData(ctx)
	if err != nil {
		return PTOutcomeOfferBackfillResult{}, err
	}
	summary, decisions := auditPTOutcomeOfferRows(finalRows, offersByKey)

	tx, err := s.DB.Pool.Begin(ctx)
	if err != nil {
		return PTOutcomeOfferBackfillResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	backfilled := 0
	skippedConflicts := 0
	for _, decision := range decisions {
		if !decision.ShouldBackfill {
			if decision.Reason == "CONFLICTING_EXISTING_OFFER" {
				skippedConflicts++
			}
			continue
		}

		catatan := "Legacy backfill from final PT"
		tag, err := tx.Exec(ctx, `
			INSERT INTO outcome_pt_offers (
				school_id,
				tahun_lulus,
				nisn,
				student_name,
				normalized_student_name,
				jalur,
				kampus,
				prodi,
				tahun_diterima,
				announcement_status,
				pilihan_ke,
				tanggal_pengumuman,
				catatan,
				source,
				updated_at
			) VALUES (
				$1,
				$2,
				$3,
				COALESCE((SELECT NULLIF(TRIM(nama_siswa),'') FROM class12_students WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3 LIMIT 1),'Tanpa Nama'),
				LOWER(COALESCE((SELECT NULLIF(TRIM(nama_siswa),'') FROM class12_students WHERE school_id=$1 AND tahun_lulus=$2 AND nisn=$3 LIMIT 1),'Tanpa Nama')),
				$4,
				$5,
				$6,
				$7,
				'DITERIMA',
				NULL,
				COALESCE($8::timestamp, NOW())::date,
				$9,
				$10,
				NOW()
			)
			ON CONFLICT DO NOTHING
		`,
			decision.FinalRow.SchoolID,
			decision.FinalRow.TahunLulus,
			decision.FinalRow.NISN,
			decision.FinalRow.Jalur,
			decision.FinalRow.Kampus,
			decision.FinalRow.Prodi,
			decision.FinalRow.TahunDiterima,
			decision.FinalRow.FinalizedAt,
			catatan,
			OutcomePTOfferSourceBackfillFromFinal,
		)
		if err != nil {
			return PTOutcomeOfferBackfillResult{}, err
		}
		if tag.RowsAffected() == 0 {
			skippedConflicts++
			logging.Warnf(
				"pt_outcome_offer_backfill",
				"insert_skipped",
				"school_id=%d tahun_lulus=%d nisn=%s jalur=%s kampus=%s prodi=%s",
				decision.FinalRow.SchoolID,
				decision.FinalRow.TahunLulus,
				decision.FinalRow.NISN,
				decision.FinalRow.Jalur,
				decision.FinalRow.Kampus,
				decision.FinalRow.Prodi,
			)
			continue
		}
		backfilled++
		logging.Infof(
			"pt_outcome_offer_backfill",
			"inserted",
			"school_id=%d tahun_lulus=%d nisn=%s jalur=%s kampus=%s prodi=%s source=%s",
			decision.FinalRow.SchoolID,
			decision.FinalRow.TahunLulus,
			decision.FinalRow.NISN,
			decision.FinalRow.Jalur,
			decision.FinalRow.Kampus,
			decision.FinalRow.Prodi,
			OutcomePTOfferSourceBackfillFromFinal,
		)
	}

	if err := tx.Commit(ctx); err != nil {
		return PTOutcomeOfferBackfillResult{}, err
	}

	result := PTOutcomeOfferBackfillResult{
		Audit:                  summary,
		Backfilled:             backfilled,
		SkippedConflictingRows: skippedConflicts,
		ProcessedAt:            time.Now(),
	}
	logging.Infof(
		"pt_outcome_offer_backfill",
		"done",
		"total_final_pt=%d total_missing_accepted_offer=%d total_accepted_detail_mismatch=%d total_backfill_candidates=%d backfilled=%d skipped_conflicting_rows=%d",
		result.Audit.TotalFinalPT,
		result.Audit.TotalMissingAcceptedOffer,
		result.Audit.TotalAcceptedDetailMismatch,
		result.Audit.TotalBackfillCandidates,
		result.Backfilled,
		result.SkippedConflictingRows,
	)
	return result, nil
}
