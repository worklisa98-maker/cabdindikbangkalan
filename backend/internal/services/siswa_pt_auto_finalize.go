package services

import (
	"context"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/logging"
)

type autoFinalizeSinglePTCandidate struct {
	StudentID     int64
	SchoolID      int
	TahunLulus    int
	NISN          string
	Jalur         string
	Kampus        string
	Prodi         string
	TahunDiterima *int
}

func (s SiswaService) StartPTAutoFinalizeWorker(ctx context.Context) {
	if s.DB == nil || s.DB.Pool == nil {
		return
	}

	run := func() {
		finalized, err := s.AutoFinalizePendingSinglePTOffers(context.Background(), time.Now())
		if err != nil {
			logging.Warnf("siswa_pt_auto_finalize", "worker", "event=failed err=%v", err)
			return
		}
		if finalized > 0 {
			logging.Infof("siswa_pt_auto_finalize", "worker", "event=finalized count=%d", finalized)
		}
	}

	go func() {
		run()

		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				run()
			}
		}
	}()
}

func (s SiswaService) tryAutoFinalizePendingSinglePTOffers(ctx context.Context) {
	if s.DB == nil || s.DB.Pool == nil {
		return
	}
	if _, err := s.AutoFinalizePendingSinglePTOffers(ctx, time.Now()); err != nil {
		logging.Warnf("siswa_pt_auto_finalize", "run", "event=failed err=%v", err)
	}
}

func (s SiswaService) AutoFinalizePendingSinglePTOffers(ctx context.Context, referenceTime time.Time) (int, error) {
	_ = referenceTime
	ok, missing, err := tablesExist(ctx, s.DB, "class12_students", "student_outcomes", "outcome_pt_offers", "pt_offer_reconciliations", "student_outcome_audit", "student_plans", "outcome_pt_details", "outcome_work_details", "outcome_business_details")
	if err != nil {
		return 0, err
	}
	if !ok {
		logging.Warnf("siswa_pt_auto_finalize", "tables", "event=missing_tables tables=%s", strings.Join(missing, ","))
		return 0, nil
	}

	candidates, err := s.fetchAutoFinalizeSinglePTCandidates(ctx)
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		return 0, nil
	}

	finalized := 0
	for _, candidate := range candidates {
		jalur, ok := normalizeJalurPenerimaan(candidate.Jalur)
		if !ok || strings.TrimSpace(candidate.Kampus) == "" || strings.TrimSpace(candidate.Prodi) == "" {
			continue
		}

		tx, err := s.DB.Pool.Begin(ctx)
		if err != nil {
			return finalized, err
		}

		_, applyErr := s.applyOutcomeTx(
			ctx,
			tx,
			candidate.SchoolID,
			candidate.NISN,
			candidate.TahunLulus,
			SiswaOutcomePayload{
				FinalStatus: "PT",
				FinalPath:   jalur,
				PtDetail: &OutcomePTDetail{
					Kampus:        strings.TrimSpace(candidate.Kampus),
					Prodi:         strings.TrimSpace(candidate.Prodi),
					Jalur:         jalur,
					TahunDiterima: candidate.TahunDiterima,
				},
				Reason: "AUTO_FINAL_PT_SINGLE_ACCEPTED",
			},
			0,
			"AUTO_FINAL_PT_SINGLE_ACCEPTED",
			OutcomeSourceFinalSiswaAkhir,
			false,
		)
		if applyErr != nil {
			_ = tx.Rollback(ctx)
			switch {
			case applyErr == ErrOutcomeFinalLocked,
				applyErr == ErrOutcomeReadOnly,
				applyErr == ErrInvalidOutcome,
				applyErr == ErrPTAnnouncementRequired,
				applyErr == ErrPTAnnouncementMismatch:
				continue
			default:
				return finalized, applyErr
			}
		}
		if err := tx.Commit(ctx); err != nil {
			return finalized, err
		}
		finalized++
	}

	return finalized, nil
}

func (s SiswaService) fetchAutoFinalizeSinglePTCandidates(ctx context.Context) ([]autoFinalizeSinglePTCandidate, error) {
	rows, err := s.DB.Pool.Query(ctx, `
		WITH accepted_single AS (
			SELECT school_id,
			       tahun_lulus,
			       nisn,
			       COUNT(*) AS accepted_count
			FROM outcome_pt_offers
			WHERE announcement_status = 'DITERIMA'
			  AND deleted_at IS NULL
			  AND COALESCE(conflict_status, 'NONE') = 'NONE'
			GROUP BY school_id, tahun_lulus, nisn
			HAVING COUNT(*) = 1
		)
		SELECT c.id,
		       c.school_id,
		       c.tahun_lulus,
		       COALESCE(c.nisn, '') AS nisn,
		       COALESCE(offer.jalur, '') AS jalur,
		       COALESCE(offer.kampus, '') AS kampus,
		       COALESCE(offer.prodi, '') AS prodi,
		       offer.tahun_diterima
		FROM class12_students c
		JOIN accepted_single accepted
		  ON accepted.school_id = c.school_id
		 AND accepted.tahun_lulus = c.tahun_lulus
		 AND accepted.nisn = c.nisn
		JOIN outcome_pt_offers offer
		  ON offer.school_id = accepted.school_id
		 AND offer.tahun_lulus = accepted.tahun_lulus
		 AND offer.nisn = accepted.nisn
		 AND offer.announcement_status = 'DITERIMA'
		 AND offer.deleted_at IS NULL
		 AND COALESCE(offer.conflict_status, 'NONE') = 'NONE'
		JOIN pt_offer_reconciliations reconciliation
		  ON reconciliation.offer_id=offer.id
		 AND reconciliation.reconciliation_status='AUTO_FINAL'
		LEFT JOIN student_outcomes o
		  ON o.school_id = c.school_id
		 AND o.tahun_lulus = c.tahun_lulus
		 AND o.nisn = c.nisn
		WHERE o.finalized_at IS NULL
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]autoFinalizeSinglePTCandidate, 0)
	for rows.Next() {
		var item autoFinalizeSinglePTCandidate
		if err := rows.Scan(
			&item.StudentID,
			&item.SchoolID,
			&item.TahunLulus,
			&item.NISN,
			&item.Jalur,
			&item.Kampus,
			&item.Prodi,
			&item.TahunDiterima,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
