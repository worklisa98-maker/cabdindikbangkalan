package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cabdin-bangkalan-backend/internal/logging"
	"github.com/jackc/pgx/v5"
)

type PTOfferReconciliationStats struct {
	Examined             int `json:"examined"`
	AutoFinal            int `json:"auto_final"`
	Pending              int `json:"pending"`
	SkippedExistingFinal int `json:"skipped_existing_final"`
	Ambiguous            int `json:"ambiguous"`
	Conflict             int `json:"conflict"`
	Failed               int `json:"failed"`
}

// reconcilePTIdentityTx records the finalization decision for one scoped
// student identity. It never rewrites student_outcomes, so an existing final
// decision always remains authoritative.
func (s SiswaService) reconcilePTIdentityTx(
	ctx context.Context,
	tx pgx.Tx,
	schoolID int,
	year int,
	normalizedName string,
) error {
	normalizedName = strings.ToLower(strings.Join(strings.Fields(normalizedName), " "))
	if schoolID <= 0 || year <= 0 || normalizedName == "" {
		return nil
	}

	lockKey := fmt.Sprintf("pt-reconcile|%d|%d|%s", schoolID, year, normalizedName)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return err
	}

	var offerCount, acceptedCount, conflictCount, acceptedDistinctPaths, distinctNISN int
	var rosterNameMatches, knownRosterMatches, existingFinalCount, crossOutcomeCount int
	if err := tx.QueryRow(ctx, `
		WITH identity_offers AS (
		  SELECT *
		  FROM outcome_pt_offers
		  WHERE school_id=$1 AND tahun_lulus=$2 AND normalized_student_name=$3
		    AND deleted_at IS NULL
		),
		known_nisn AS (
		  SELECT DISTINCT TRIM(nisn) AS nisn
		  FROM identity_offers
		  WHERE NULLIF(TRIM(COALESCE(nisn,'')), '') IS NOT NULL
		),
		roster_matches AS (
		  SELECT c.nisn
		  FROM class12_students c
		  WHERE c.school_id=$1 AND c.tahun_lulus=$2
		    AND LOWER(REGEXP_REPLACE(TRIM(COALESCE(c.nama_siswa,'')), '\s+', ' ', 'g'))=$3
		    AND `+activeClass12StudentCondition("c")+`
		)
		SELECT
		  (SELECT COUNT(*) FROM identity_offers),
		  (SELECT COUNT(*) FROM identity_offers WHERE announcement_status='DITERIMA'),
		  (SELECT COUNT(*) FROM identity_offers WHERE COALESCE(conflict_status,'NONE')<>'NONE'),
		  (SELECT COUNT(DISTINCT UPPER(TRIM(jalur))) FROM identity_offers WHERE announcement_status='DITERIMA'),
		  (SELECT COUNT(*) FROM known_nisn),
		  (SELECT COUNT(*) FROM roster_matches),
		  (
		    SELECT COUNT(*)
		    FROM known_nisn known
		    JOIN roster_matches roster ON roster.nisn=known.nisn
		  ),
		  (
		    SELECT COUNT(*)
		    FROM student_outcomes o
		    WHERE o.school_id=$1 AND o.tahun_lulus=$2 AND o.finalized_at IS NOT NULL
		      AND (
		        o.nisn IN (SELECT nisn FROM known_nisn)
		        OR o.nisn IN (SELECT nisn FROM roster_matches)
		      )
		  ),
		  (
		    SELECT COUNT(*)
		    FROM (
		      SELECT nisn FROM outcome_work_details
		      WHERE school_id=$1 AND tahun_lulus=$2
		      UNION ALL
		      SELECT nisn FROM outcome_business_details
		      WHERE school_id=$1 AND tahun_lulus=$2
		    ) other
		    WHERE other.nisn IN (SELECT nisn FROM known_nisn)
		       OR other.nisn IN (SELECT nisn FROM roster_matches)
		  )
	`, schoolID, year, normalizedName).Scan(
		&offerCount, &acceptedCount, &conflictCount, &acceptedDistinctPaths, &distinctNISN,
		&rosterNameMatches, &knownRosterMatches, &existingFinalCount, &crossOutcomeCount,
	); err != nil {
		return err
	}
	if offerCount == 0 {
		return nil
	}

	status := "PENDING"
	reason := "MULTIPLE_OR_CONFLICTING_OFFERS"
	switch {
	case existingFinalCount > 0:
		status, reason = "EXISTING_FINAL", "EXISTING_FINAL_PRESERVED"
	case acceptedCount == 0:
		status, reason = "NOT_ACCEPTED", "NO_ACCEPTED_OFFER"
	case acceptedCount != 1 || acceptedDistinctPaths != 1:
		status, reason = "PENDING", "MULTIPLE_ACCEPTANCE_CANDIDATES"
	case conflictCount > 0:
		status, reason = "PENDING", "OFFER_CONFLICT"
	case distinctNISN > 1:
		status, reason = "PENDING", "NISN_CONFLICT"
	case distinctNISN > 0 && rosterNameMatches > 0 && knownRosterMatches == 0:
		status, reason = "PENDING", "NISN_ROSTER_CONFLICT"
	case distinctNISN == 0 && rosterNameMatches > 1:
		status, reason = "PENDING", "AMBIGUOUS_NORMALIZED_NAME"
	case crossOutcomeCount > 0:
		status, reason = "PENDING", "CROSS_OUTCOME_CONFLICT"
	default:
		status, reason = "AUTO_FINAL", "SINGLE_ACCEPTED_OFFER"
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO pt_offer_reconciliations(
		  offer_id,school_id,tahun_lulus,identity_key,reconciliation_status,reason,reconciled_at
		)
		SELECT id,school_id,tahun_lulus,
		  COALESCE(NULLIF(TRIM(nisn),''),'NAME:'||normalized_student_name),
		  $4,$5,NOW()
		FROM outcome_pt_offers
		WHERE school_id=$1 AND tahun_lulus=$2 AND normalized_student_name=$3
		  AND deleted_at IS NULL
		ON CONFLICT (offer_id) DO UPDATE SET
		  school_id=EXCLUDED.school_id,
		  tahun_lulus=EXCLUDED.tahun_lulus,
		  identity_key=EXCLUDED.identity_key,
		  reconciliation_status=EXCLUDED.reconciliation_status,
		  reason=EXCLUDED.reason,
		  reconciled_at=NOW()
	`, schoolID, year, normalizedName, status, reason)
	return err
}

// ReconcilePTIdentityTx exposes the scoped transactional operation to
// handlers that already own a mutation transaction (for example cancellation).
func (s SiswaService) ReconcilePTIdentityTx(ctx context.Context, tx pgx.Tx, schoolID, year int, normalizedName string) error {
	return s.reconcilePTIdentityTx(ctx, tx, schoolID, year, normalizedName)
}

// ReconcileLegacyPTOffers evaluates old and stale offers with the exact same
// identity-level service used by live mutations. It is safe to rerun: the
// reconciliation row is keyed by offer_id and only missing/stale identities
// are selected.
func (s SiswaService) ReconcileLegacyPTOffers(ctx context.Context, batchSize int) PTOfferReconciliationStats {
	stats := PTOfferReconciliationStats{}
	if s.DB == nil || s.DB.Pool == nil {
		return stats
	}
	if batchSize <= 0 || batchSize > 1000 {
		batchSize = 200
	}
	for {
		rows, err := s.DB.Pool.Query(ctx, `
			SELECT offer.school_id,offer.tahun_lulus,offer.normalized_student_name,COUNT(*)::int
			FROM outcome_pt_offers offer
			LEFT JOIN pt_offer_reconciliations reconciliation ON reconciliation.offer_id=offer.id
			WHERE offer.deleted_at IS NULL
			  AND NULLIF(TRIM(COALESCE(offer.normalized_student_name,'')), '') IS NOT NULL
			  AND (reconciliation.offer_id IS NULL OR reconciliation.reconciled_at < offer.updated_at)
			GROUP BY offer.school_id,offer.tahun_lulus,offer.normalized_student_name
			ORDER BY offer.school_id,offer.tahun_lulus,offer.normalized_student_name
			LIMIT $1
		`, batchSize)
		if err != nil {
			stats.Failed++
			return stats
		}
		type identity struct {
			schoolID, year, offers int
			name                   string
		}
		batch := make([]identity, 0, batchSize)
		for rows.Next() {
			var item identity
			if err := rows.Scan(&item.schoolID, &item.year, &item.name, &item.offers); err != nil {
				stats.Failed++
				continue
			}
			batch = append(batch, item)
		}
		rows.Close()
		if len(batch) == 0 {
			return stats
		}
		batchFailed := false
		for _, item := range batch {
			stats.Examined += item.offers
			tx, err := s.DB.Pool.Begin(ctx)
			if err != nil {
				stats.Failed += item.offers
				batchFailed = true
				continue
			}
			if err = s.reconcilePTIdentityTx(ctx, tx, item.schoolID, item.year, item.name); err != nil {
				_ = tx.Rollback(ctx)
				stats.Failed += item.offers
				batchFailed = true
				continue
			}
			var status, reason string
			if err = tx.QueryRow(ctx, `
				SELECT reconciliation_status,reason FROM pt_offer_reconciliations
				WHERE school_id=$1 AND tahun_lulus=$2
				  AND offer_id IN (SELECT id FROM outcome_pt_offers WHERE school_id=$1 AND tahun_lulus=$2 AND normalized_student_name=$3 AND deleted_at IS NULL)
				ORDER BY reconciled_at DESC LIMIT 1
			`, item.schoolID, item.year, item.name).Scan(&status, &reason); err != nil {
				_ = tx.Rollback(ctx)
				stats.Failed += item.offers
				batchFailed = true
				continue
			}
			if err = tx.Commit(ctx); err != nil {
				stats.Failed += item.offers
				batchFailed = true
				continue
			}
			switch status {
			case "AUTO_FINAL":
				stats.AutoFinal += item.offers
			case "EXISTING_FINAL":
				stats.SkippedExistingFinal += item.offers
			case "PENDING":
				stats.Pending += item.offers
			}
			if strings.Contains(reason, "AMBIGUOUS") {
				stats.Ambiguous += item.offers
			}
			if strings.Contains(reason, "CONFLICT") {
				stats.Conflict += item.offers
			}
		}
		if batchFailed {
			return stats
		}
	}
}

func (s SiswaService) StartPTOfferReconciliationWorker(ctx context.Context) {
	if s.DB == nil || s.DB.Pool == nil {
		return
	}
	run := func() {
		stats := s.ReconcileLegacyPTOffers(context.Background(), 200)
		logging.Infof("siswa_pt_reconciliation", "worker",
			"examined=%d auto_final=%d pending=%d existing_final=%d ambiguous=%d conflict=%d failed=%d",
			stats.Examined, stats.AutoFinal, stats.Pending, stats.SkippedExistingFinal,
			stats.Ambiguous, stats.Conflict, stats.Failed)
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
