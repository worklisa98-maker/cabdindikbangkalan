package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"cabdin-bangkalan-backend/internal/auth"
)

func newGaspolPreviewID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return hex.EncodeToString(raw[0:4]) + "-" + hex.EncodeToString(raw[4:6]) + "-" +
		hex.EncodeToString(raw[6:8]) + "-" + hex.EncodeToString(raw[8:10]) + "-" +
		hex.EncodeToString(raw[10:16]), nil
}

func (s GaspolService) PreviewRandomAssignments(ctx context.Context, user auth.User, year, month int) (GaspolAssignmentPreview, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolAssignmentPreview{}, err
	}
	year = normalizeGaspolYear(year)
	month = normalizeGaspolMonth(month)
	if month < 4 || month > 9 {
		return GaspolAssignmentPreview{}, errors.New("INVALID_MONTH")
	}
	schools, supervisors, err := s.Repo.ListRandomAssignmentInputs(ctx, year, month)
	if err != nil {
		return GaspolAssignmentPreview{}, err
	}
	existing, err := s.Repo.ListAssignments(ctx, year, month)
	if err != nil {
		return GaspolAssignmentPreview{}, err
	}
	existingBySchool := make(map[int]GaspolAssignment, len(existing))
	for _, item := range existing {
		existingBySchool[item.SchoolID] = item
	}
	schoolNames := make(map[int]string, len(schools))
	mutable := make([]GaspolAssignmentCandidate, 0, len(schools))
	locked := make([]GaspolAssignment, 0)
	lockedLoads := map[int]int{}
	for _, school := range schools {
		schoolNames[school.SchoolID] = school.SchoolName
		isLocked, lockErr := s.Repo.HasSubmittedSupervisorAssessment(ctx, year, month, school.SchoolID)
		if lockErr != nil {
			return GaspolAssignmentPreview{}, lockErr
		}
		if isLocked {
			if item, ok := existingBySchool[school.SchoolID]; ok {
				locked = append(locked, item)
				if item.SupervisorUserID != nil {
					lockedLoads[*item.SupervisorUserID]++
				}
			}
			continue
		}
		mutable = append(mutable, school)
	}
	for index := range supervisors {
		supervisors[index].CurrentLoad = lockedLoads[supervisors[index].UserID]
	}
	plans := BuildGaspolRandomAssignments(year, month, mutable, supervisors)
	previewID, err := newGaspolPreviewID()
	if err != nil {
		return GaspolAssignmentPreview{}, err
	}
	preview := GaspolAssignmentPreview{
		PreviewID: previewID, Year: year, Month: month, Assignments: plans,
		LockedAssignments: locked, UnassignedSchoolIDs: make([]int, 0),
	}
	rowsBySupervisor := map[int]*GaspolAssignmentPreviewRow{}
	for _, supervisor := range supervisors {
		rowsBySupervisor[supervisor.UserID] = &GaspolAssignmentPreviewRow{
			SupervisorUserID: supervisor.UserID, SupervisorName: supervisor.DisplayName,
			PreviousMonthCount: supervisor.PreviousMonthLoad, HistoricalCount: supervisor.HistoricalLoad,
			SchoolNames: make([]string, 0),
		}
	}
	for _, item := range locked {
		if item.SupervisorUserID == nil {
			preview.UnassignedSchoolIDs = append(preview.UnassignedSchoolIDs, item.SchoolID)
			continue
		}
		row := rowsBySupervisor[*item.SupervisorUserID]
		if row == nil {
			continue
		}
		row.CurrentMonthCount++
		row.LockedCount++
		if item.Method == GaspolAssignmentManual {
			row.ManualCount++
		}
		row.SchoolNames = append(row.SchoolNames, item.SchoolName)
	}
	for _, plan := range plans {
		if plan.SupervisorUserID == nil {
			preview.UnassignedSchoolIDs = append(preview.UnassignedSchoolIDs, plan.SchoolID)
			continue
		}
		row := rowsBySupervisor[*plan.SupervisorUserID]
		if row == nil {
			continue
		}
		row.CurrentMonthCount++
		row.NewCount++
		row.SchoolNames = append(row.SchoolNames, schoolNames[plan.SchoolID])
	}
	for _, row := range rowsBySupervisor {
		row.RunningTotal = row.HistoricalCount + row.CurrentMonthCount
		sort.Strings(row.SchoolNames)
		preview.Rows = append(preview.Rows, *row)
	}
	sort.Slice(preview.Rows, func(i, j int) bool { return preview.Rows[i].SupervisorName < preview.Rows[j].SupervisorName })
	payload, err := json.Marshal(preview)
	if err != nil {
		return GaspolAssignmentPreview{}, err
	}
	_, err = s.Repo.DB.Pool.Exec(ctx, `
		INSERT INTO gaspol_assignment_previews (id,year,month,created_by,payload,expires_at)
		VALUES ($1,$2,$3,$4,$5,NOW()+INTERVAL '30 minutes')
	`, previewID, year, month, user.ID, payload)
	return preview, err
}

func (s GaspolService) CommitRandomAssignmentPreview(ctx context.Context, user auth.User, previewID string) (GaspolAssignmentGenerateResult, error) {
	if err := s.ensureAdminMaster(user); err != nil {
		return GaspolAssignmentGenerateResult{}, err
	}
	var payload []byte
	var committedAt *time.Time
	err := s.Repo.DB.Pool.QueryRow(ctx, `
		SELECT payload,committed_at FROM gaspol_assignment_previews
		WHERE id=$1 AND created_by=$2 AND expires_at>NOW()
	`, previewID, user.ID).Scan(&payload, &committedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return GaspolAssignmentGenerateResult{}, errors.New("PREVIEW_EXPIRED")
	}
	if err != nil {
		return GaspolAssignmentGenerateResult{}, err
	}
	if committedAt != nil {
		return GaspolAssignmentGenerateResult{}, errors.New("PREVIEW_ALREADY_COMMITTED")
	}
	var preview GaspolAssignmentPreview
	if err := json.Unmarshal(payload, &preview); err != nil {
		return GaspolAssignmentGenerateResult{}, err
	}
	assignments := append([]GaspolAssignment(nil), preview.LockedAssignments...)
	for _, locked := range preview.LockedAssignments {
		isLocked, lockErr := s.Repo.HasSubmittedSupervisorAssessment(ctx, preview.Year, preview.Month, locked.SchoolID)
		if lockErr != nil {
			return GaspolAssignmentGenerateResult{}, lockErr
		}
		if !isLocked {
			return GaspolAssignmentGenerateResult{}, errors.New("PREVIEW_STALE")
		}
	}
	for _, plan := range preview.Assignments {
		isLocked, lockErr := s.Repo.HasSubmittedSupervisorAssessment(ctx, preview.Year, preview.Month, plan.SchoolID)
		if lockErr != nil {
			return GaspolAssignmentGenerateResult{}, lockErr
		}
		if isLocked {
			return GaspolAssignmentGenerateResult{}, errors.New("PREVIEW_STALE")
		}
		item, upsertErr := s.Repo.UpsertAssignment(ctx, preview.Year, preview.Month, plan.SchoolID,
			plan.SupervisorUserID, plan.SupervisorNameSnapshot, plan.Method, user.ID)
		if upsertErr != nil {
			return GaspolAssignmentGenerateResult{}, upsertErr
		}
		assignments = append(assignments, item)
	}
	tag, err := s.Repo.DB.Pool.Exec(ctx, `UPDATE gaspol_assignment_previews SET committed_at=NOW() WHERE id=$1 AND committed_at IS NULL`, previewID)
	if err != nil {
		return GaspolAssignmentGenerateResult{}, err
	}
	if tag.RowsAffected() != 1 {
		return GaspolAssignmentGenerateResult{}, errors.New("PREVIEW_ALREADY_COMMITTED")
	}
	result := GaspolAssignmentGenerateResult{
		Year: preview.Year, Month: preview.Month, Assignments: ensureGaspolAssignments(assignments),
		UnassignedSchoolIDs: preview.UnassignedSchoolIDs,
	}
	_ = s.Repo.LogAudit(ctx, &preview.Year, user.ID, user.Role, "RANDOM_ASSIGNMENT_CONFIRM", "gaspol_assignments", nil, nil, result)
	return result, nil
}
