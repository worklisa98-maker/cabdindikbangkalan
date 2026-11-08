package services

import (
	"errors"
	"sort"
	"strings"
)

type GaspolCompetitionCandidate struct {
	SchoolID          int     `json:"schoolId"`
	SchoolName        string  `json:"schoolName"`
	SchoolNPSN        string  `json:"schoolNpsn,omitempty"`
	MonthlyScore      float64 `json:"monthlyScore"`
	FieldStage1Score  float64 `json:"fieldStage1Score"`
	FieldStage2Score  float64 `json:"fieldStage2Score"`
	PresentationScore float64 `json:"presentationScore"`
	NominationScore   float64 `json:"nominationScore"`
	AppealScore       float64 `json:"appealScore"`
	Total             float64 `json:"total"`
	Maximum           float64 `json:"maximum"`
	Rank              int     `json:"rank"`
}

func RankGaspolCompetition(candidates []GaspolCompetitionCandidate, stage string) []GaspolCompetitionCandidate {
	rows := append([]GaspolCompetitionCandidate(nil), candidates...)
	for i := range rows {
		input := GaspolScoringInput{
			MonthlyComponents: completeMonthlyComponentsForRanking(rows[i].MonthlyScore),
			FieldStage1Score:  rows[i].FieldStage1Score, FieldStage2Score: rows[i].FieldStage2Score,
			PresentationScore: rows[i].PresentationScore, NominationScore: rows[i].NominationScore, AppealScore: rows[i].AppealScore,
		}
		score := ComputeGaspolStageScore(stage, input)
		rows[i].Total = score.ScoreTotal
		rows[i].Maximum = score.MaximumScore
	}
	sort.SliceStable(rows, func(i, j int) bool { return gaspolCandidateLess(rows[i], rows[j], stage) })
	for i := range rows {
		if i == 0 {
			rows[i].Rank = 1
			continue
		}
		if gaspolCandidatesEqualForStage(rows[i-1], rows[i], stage) {
			rows[i].Rank = rows[i-1].Rank
		} else {
			rows[i].Rank = i + 1
		}
	}
	return rows
}

func completeMonthlyComponentsForRanking(score float64) []GaspolMonthlyComponent {
	rows := gaspolMonthlyComponentDefinitions()
	for i := range rows {
		rows[i].Score = score
		rows[i].IsFinal = true
	}
	return rows
}

func gaspolCandidateLess(a, b GaspolCompetitionCandidate, stage string) bool {
	if a.Total != b.Total {
		return a.Total > b.Total
	}
	switch stage {
	case GaspolStageField, GaspolStageField1:
		if a.FieldStage1Score != b.FieldStage1Score {
			return a.FieldStage1Score > b.FieldStage1Score
		}
	case GaspolStageField2:
		if a.FieldStage2Score != b.FieldStage2Score {
			return a.FieldStage2Score > b.FieldStage2Score
		}
		if a.FieldStage1Score != b.FieldStage1Score {
			return a.FieldStage1Score > b.FieldStage1Score
		}
	case GaspolStagePresentation, GaspolStageFinal:
		if a.PresentationScore != b.PresentationScore {
			return a.PresentationScore > b.PresentationScore
		}
		if a.FieldStage2Score != b.FieldStage2Score {
			return a.FieldStage2Score > b.FieldStage2Score
		}
		if a.FieldStage1Score != b.FieldStage1Score {
			return a.FieldStage1Score > b.FieldStage1Score
		}
	case GaspolStageAppeal:
		if a.AppealScore != b.AppealScore {
			return a.AppealScore > b.AppealScore
		}
	}
	if a.MonthlyScore != b.MonthlyScore {
		return a.MonthlyScore > b.MonthlyScore
	}
	// Deterministic presentation order is not a competition decision; equal rows retain equal rank.
	return strings.ToLower(a.SchoolName) < strings.ToLower(b.SchoolName)
}

func gaspolCandidatesEqualForStage(a, b GaspolCompetitionCandidate, stage string) bool {
	if a.Total != b.Total {
		return false
	}
	switch stage {
	case GaspolStageField, GaspolStageField1:
		return a.FieldStage1Score == b.FieldStage1Score && a.MonthlyScore == b.MonthlyScore
	case GaspolStageField2:
		return a.FieldStage2Score == b.FieldStage2Score && a.FieldStage1Score == b.FieldStage1Score && a.MonthlyScore == b.MonthlyScore
	case GaspolStagePresentation, GaspolStageFinal:
		return a.PresentationScore == b.PresentationScore && a.FieldStage2Score == b.FieldStage2Score && a.FieldStage1Score == b.FieldStage1Score
	case GaspolStageAppeal:
		return a.AppealScore == b.AppealScore && a.MonthlyScore == b.MonthlyScore && a.FieldStage1Score == b.FieldStage1Score && a.NominationScore == b.NominationScore
	default:
		return a.MonthlyScore == b.MonthlyScore
	}
}

func RequireGaspolBoundaryResolution(rows []GaspolCompetitionCandidate, limit int, stage string, manualSchoolID *int) error {
	if limit <= 0 || len(rows) <= limit {
		return nil
	}
	left, right := rows[limit-1], rows[limit]
	if !gaspolCandidatesEqualForStage(left, right, stage) {
		return nil
	}
	if manualSchoolID == nil || (*manualSchoolID != left.SchoolID && *manualSchoolID != right.SchoolID) {
		return errors.New("MANUAL_TIE_RESOLUTION_REQUIRED")
	}
	return nil
}

func SelectGaspolBottom(rows []GaspolCompetitionCandidate, limit int) []GaspolCompetitionCandidate {
	if limit <= 0 {
		return []GaspolCompetitionCandidate{}
	}
	copyRows := append([]GaspolCompetitionCandidate(nil), rows...)
	sort.SliceStable(copyRows, func(i, j int) bool {
		if copyRows[i].Total == copyRows[j].Total {
			return strings.ToLower(copyRows[i].SchoolName) < strings.ToLower(copyRows[j].SchoolName)
		}
		return copyRows[i].Total < copyRows[j].Total
	})
	if len(copyRows) < limit {
		limit = len(copyRows)
	}
	return copyRows[:limit]
}
