package services

import (
	"math"
	"sort"
	"strings"
	"time"
)

const (
	gaspolMonthlyWeight        = 0.20
	gaspolFieldStage1Weight    = 0.40
	gaspolFieldStage2Weight    = 0.20
	gaspolPresentationWeight   = 0.20
	gaspolNominationWeight     = 0.30
	gaspolAppealWeight         = 0.10
	gaspolInternalVoteWeight   = 0.60
	gaspolContentLikeWeight    = 0.40
	gaspolDefaultLikeThreshold = 800.0
)

type GaspolScoringInput struct {
	MonthlyComponents []GaspolMonthlyComponent
	// FieldScore is kept as the legacy API alias for FieldStage1Score.
	FieldScore        float64
	FieldStage1Score  float64
	FieldStage2Score  float64
	PresentationScore float64
	NominationScore   float64
	AppealScore       float64
	HasFieldStage1    bool
	HasFieldStage2    bool
	HasPresentation   bool
	HasNomination     bool
	HasAppeal         bool
}

func gaspolFloatPointer(value float64) *float64 {
	value = roundGaspolScore(value)
	return &value
}

func gaspolContribution(key, label string, score float64, present bool, weight float64) GaspolScoreContribution {
	item := GaspolScoreContribution{Key: key, Label: label, Weight: weight, Status: "NOT_ASSESSED"}
	if present {
		item.Score = gaspolFloatPointer(score)
		item.WeightedScore = gaspolFloatPointer(score * weight)
		item.Status = "FINAL"
	}
	return item
}

func GaspolScoreContributions(input GaspolScoringInput, path string) []GaspolScoreContribution {
	monthly := calculateGaspolMonthlyWeightedScore(input.MonthlyComponents)
	result := []GaspolScoreContribution{gaspolContribution("MONTHLY", "Nilai Bulanan", monthly, true, gaspolMonthlyWeight)}
	hasField1 := input.HasFieldStage1 || input.FieldStage1Score != 0 || input.FieldScore != 0
	hasField2 := input.HasFieldStage2 || input.FieldStage2Score != 0
	hasPresentation := input.HasPresentation || input.PresentationScore != 0
	hasNomination := input.HasNomination || input.NominationScore != 0
	hasAppeal := input.HasAppeal || input.AppealScore != 0
	result = append(result, gaspolContribution("FIELD_STAGE_1", "Lapangan Tahap 1", gaspolFieldStage1Score(input), hasField1, gaspolFieldStage1Weight))
	if strings.EqualFold(path, "NOT_FAILED") {
		result = append(result,
			gaspolContribution("NOT_FAILED_NOMINATION", "Nominasi Tidak Tergagal", input.NominationScore, hasNomination, gaspolNominationWeight),
			gaspolContribution("BOTTOM5_APPEAL", "Banding Kandidat Zona Tergagal", input.AppealScore, hasAppeal, gaspolAppealWeight),
		)
		return result
	}
	return append(result,
		gaspolContribution("FIELD_STAGE_2", "Lapangan Tahap 2", input.FieldStage2Score, hasField2, gaspolFieldStage2Weight),
		gaspolContribution("PRESENTATION", "Presentasi", input.PresentationScore, hasPresentation, gaspolPresentationWeight),
	)
}

func GaspolCurrentMaximum(input GaspolScoringInput, path string) float64 {
	maximum := 20.0
	if input.HasFieldStage1 || input.FieldStage1Score != 0 || input.FieldScore != 0 {
		maximum = 60
	}
	if strings.EqualFold(path, "NOT_FAILED") {
		if input.HasNomination || input.NominationScore != 0 {
			maximum = 90
		}
		if input.HasAppeal || input.AppealScore != 0 {
			maximum = 100
		}
		return maximum
	}
	if input.HasFieldStage2 || input.FieldStage2Score != 0 {
		maximum = 80
	}
	if input.HasPresentation || input.PresentationScore != 0 {
		maximum = 100
	}
	return maximum
}

type GaspolFavoriteCandidate struct {
	SchoolID         int
	SchoolName       string
	Votes            int
	VoterDenominator int
	ActualLikes      int
	LikeThreshold    int
	Score            GaspolFavoriteScore
}

func RankGaspolFavoriteCandidates(input []GaspolFavoriteCandidate) []GaspolFavoriteCandidate {
	rows := append([]GaspolFavoriteCandidate(nil), input...)
	for i := range rows {
		rows[i].Score = ComputeGaspolFavoriteScore(rows[i].Votes, rows[i].VoterDenominator, rows[i].ActualLikes, rows[i].LikeThreshold)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Score.Total != rows[j].Score.Total {
			return rows[i].Score.Total > rows[j].Score.Total
		}
		if rows[i].Score.InternalScore != rows[j].Score.InternalScore {
			return rows[i].Score.InternalScore > rows[j].Score.InternalScore
		}
		if rows[i].ActualLikes != rows[j].ActualLikes {
			return rows[i].ActualLikes > rows[j].ActualLikes
		}
		return strings.ToLower(rows[i].SchoolName) < strings.ToLower(rows[j].SchoolName)
	})
	return rows
}

func ComputeGaspolScore(input GaspolScoringInput) GaspolScoreBreakdown {
	monthly := calculateGaspolMonthlyWeightedScore(input.MonthlyComponents)
	field := gaspolFieldStage1Score(input)
	fieldStage2 := sanitizeGaspolScore(input.FieldStage2Score)
	presentation := sanitizeGaspolScore(input.PresentationScore)
	total := roundGaspolScore((monthly * gaspolMonthlyWeight) + (field * gaspolFieldStage1Weight) + (fieldStage2 * gaspolFieldStage2Weight) + (presentation * gaspolPresentationWeight))
	maximum := GaspolCurrentMaximum(input, "MAIN")

	return GaspolScoreBreakdown{
		ScoreMonthly:      monthly,
		ScoreField:        field,
		ScoreFieldStage1:  field,
		ScoreFieldStage2:  fieldStage2,
		ScorePresentation: presentation,
		ScoreTotal:        total,
		MaximumScore:      maximum,
		CurrentMaxScore:   maximum,
		Contributions:     GaspolScoreContributions(input, "MAIN"),
	}
}

func ComputeGaspolStageScore(stage string, input GaspolScoringInput) GaspolScoreBreakdown {
	monthly := calculateGaspolMonthlyWeightedScore(input.MonthlyComponents)
	field := gaspolFieldStage1Score(input)
	fieldStage2 := sanitizeGaspolScore(input.FieldStage2Score)
	presentation := sanitizeGaspolScore(input.PresentationScore)

	total := 0.0
	switch stage {
	case GaspolStageField, GaspolStageField1:
		total = (monthly * gaspolMonthlyWeight) + (field * gaspolFieldStage1Weight)
	case GaspolStageField2:
		total = (monthly * gaspolMonthlyWeight) + (field * gaspolFieldStage1Weight) + (fieldStage2 * gaspolFieldStage2Weight)
	case GaspolStagePresentation, GaspolStageFinal:
		total = (monthly * gaspolMonthlyWeight) + (field * gaspolFieldStage1Weight) + (fieldStage2 * gaspolFieldStage2Weight) + (presentation * gaspolPresentationWeight)
	case GaspolStageNomination:
		total = (monthly * gaspolMonthlyWeight) + (field * gaspolFieldStage1Weight) + (sanitizeGaspolScore(input.NominationScore) * gaspolNominationWeight)
	case GaspolStageAppeal:
		total = (monthly * gaspolMonthlyWeight) + (field * gaspolFieldStage1Weight) + (sanitizeGaspolScore(input.NominationScore) * gaspolNominationWeight) + (sanitizeGaspolScore(input.AppealScore) * gaspolAppealWeight)
	default:
		total = monthly * gaspolMonthlyWeight
	}

	return GaspolScoreBreakdown{
		ScoreMonthly:      monthly,
		ScoreField:        field,
		ScoreFieldStage1:  field,
		ScoreFieldStage2:  fieldStage2,
		ScorePresentation: presentation,
		ScoreNomination:   sanitizeGaspolScore(input.NominationScore),
		ScoreAppeal:       sanitizeGaspolScore(input.AppealScore),
		ScoreTotal:        roundGaspolScore(total),
		MaximumScore:      GaspolMaximumForStage(stage),
		CurrentMaxScore:   GaspolMaximumForStage(stage),
		Contributions:     GaspolScoreContributions(input, map[bool]string{true: "NOT_FAILED", false: "MAIN"}[stage == GaspolStageNomination || stage == GaspolStageAppeal]),
	}
}

func gaspolFieldStage1Score(input GaspolScoringInput) float64 {
	if input.FieldStage1Score != 0 || input.FieldScore == 0 {
		return sanitizeGaspolScore(input.FieldStage1Score)
	}
	return sanitizeGaspolScore(input.FieldScore)
}

func GaspolMaximumForStage(stage string) float64 {
	switch stage {
	case GaspolStageField, GaspolStageField1:
		return 60
	case GaspolStageField2:
		return 80
	case GaspolStageNomination:
		return 90
	case GaspolStagePresentation, GaspolStageAppeal, GaspolStageFinal:
		return 100
	default:
		return 20
	}
}

func ComputeGaspolFavoriteScore(votes, voterDenominator, actualLikes, likeThreshold int) GaspolFavoriteScore {
	internal := 0.0
	if voterDenominator > 0 {
		internal = math.Min(float64(votes)/float64(voterDenominator)*100, 100)
	}
	threshold := float64(likeThreshold)
	if threshold <= 0 {
		threshold = gaspolDefaultLikeThreshold
	}
	likes := math.Min(float64(maxInt(actualLikes, 0))/threshold*100, 100)
	return GaspolFavoriteScore{
		InternalScore: roundGaspolScore(internal),
		LikeScore:     roundGaspolScore(likes),
		ActualLikes:   maxInt(actualLikes, 0),
		Total:         roundGaspolScore(internal*gaspolInternalVoteWeight + likes*gaspolContentLikeWeight),
	}
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func ScoreValueForStage(stage string, score float64) (float64, float64, float64) {
	value := sanitizeGaspolScore(score)
	switch stage {
	case GaspolStageMonthly:
		return value, 0, 0
	case GaspolStageField:
		return 0, value, 0
	case GaspolStageField1, GaspolStageField2:
		return 0, value, 0
	case GaspolStagePresentation:
		return 0, 0, value
	default:
		return 0, 0, 0
	}
}

func roundGaspolScore(value float64) float64 {
	return math.Round(value*100) / 100
}

func sanitizeGaspolScore(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return roundGaspolScore(value)
}

func gaspolMonthlyComponentDefinitions() []GaspolMonthlyComponent {
	return []GaspolMonthlyComponent{
		{Month: 4, Label: "April", Weight: 0.20},
		{Month: 5, Label: "Mei", Weight: 0.15},
		{Month: 6, Label: "Juni", Weight: 0.20},
		{Month: 7, Label: "Juli", Weight: 0.15},
		{Month: 8, Label: "Agustus", Weight: 0.15},
		{Month: 9, Label: "September", Weight: 0.15},
	}
}

func isGaspolMonthlyComponentMonth(month int) bool {
	for _, item := range gaspolMonthlyComponentDefinitions() {
		if item.Month == month {
			return true
		}
	}
	return false
}

func gaspolMonthlyComponentWeight(month int) float64 {
	for _, item := range gaspolMonthlyComponentDefinitions() {
		if item.Month == month {
			return item.Weight
		}
	}
	return 0
}

func gaspolMonthlyComponentStatusLabel(status string) string {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case GaspolMonthlyStatusValidated:
		return "Tervalidasi"
	case GaspolMonthlyStatusWaiting:
		return "Waiting"
	case GaspolMonthlyStatusZeroFinal:
		return "0 Final"
	case GaspolMonthlyStatusNotYetDue:
		return "Belum Waktunya"
	default:
		return "Belum Final"
	}
}

func gaspolMonthlyComponentStatusIsFinal(status string) bool {
	switch strings.ToUpper(strings.TrimSpace(status)) {
	case GaspolMonthlyStatusValidated, GaspolMonthlyStatusZeroFinal:
		return true
	default:
		return false
	}
}

func calculateGaspolMonthlyWeightedScore(components []GaspolMonthlyComponent) float64 {
	componentByMonth := make(map[int]GaspolMonthlyComponent, len(components))
	for _, item := range components {
		if !isGaspolMonthlyComponentMonth(item.Month) {
			continue
		}
		componentByMonth[item.Month] = item
	}

	total := 0.0
	for _, definition := range gaspolMonthlyComponentDefinitions() {
		score := 0.0
		if item, ok := componentByMonth[definition.Month]; ok {
			score = sanitizeGaspolScore(item.Score)
		}
		total += score * definition.Weight
	}
	return roundGaspolScore(total)
}

func GaspolDeadlineForMonth(year int, month int) time.Time {
	return GaspolDeadlineForMonthInLocation(year, month, JakartaLocation())
}

func GaspolDeadlineForMonthInLocation(year int, month int, location *time.Location) time.Time {
	nextMonth := month + 1
	nextYear := year
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}
	if location == nil {
		location = JakartaLocation()
	}
	return time.Date(nextYear, time.Month(nextMonth), 15, 23, 59, 59, 0, location)
}

func IsGaspolDeadlineOpen(year int, month int, now time.Time) bool {
	return CanEditGaspolMonthlyEntry(year, month, now)
}

func IsGaspolMonthlyEntryLocked(year int, month int, effectiveNow time.Time) bool {
	if month < 4 || month > 11 {
		return true
	}
	deadline := GaspolDeadlineForMonthInLocation(year, month, JakartaLocation())
	return normalizeJakartaTime(effectiveNow).After(deadline)
}

func CanEditGaspolMonthlyEntry(year int, month int, effectiveNow time.Time) bool {
	return !IsGaspolMonthlyEntryLocked(year, month, effectiveNow)
}

func RankGaspolRows(rows []GaspolRankingRow) []GaspolRankingRow {
	if len(rows) == 0 {
		return []GaspolRankingRow{}
	}

	ranked := append([]GaspolRankingRow(nil), rows...)
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Scores.ScoreTotal == ranked[j].Scores.ScoreTotal {
			if ranked[i].Scores.ScoreField == ranked[j].Scores.ScoreField {
				if ranked[i].Scores.ScoreMonthly == ranked[j].Scores.ScoreMonthly {
					return strings.ToLower(ranked[i].SchoolName) < strings.ToLower(ranked[j].SchoolName)
				}
				return ranked[i].Scores.ScoreMonthly > ranked[j].Scores.ScoreMonthly
			}
			return ranked[i].Scores.ScoreField > ranked[j].Scores.ScoreField
		}
		return ranked[i].Scores.ScoreTotal > ranked[j].Scores.ScoreTotal
	})

	currentRank := 0
	lastScore := -1.0
	for index := range ranked {
		if index == 0 || ranked[index].Scores.ScoreTotal != lastScore {
			currentRank = index + 1
			lastScore = ranked[index].Scores.ScoreTotal
		}
		ranked[index].Rank = currentRank
	}

	return ranked
}

func SelectGaspolTopTen(rows []GaspolRankingRow) []GaspolRankingRow {
	return selectGaspolTopRows(rows, 10)
}

func SelectGaspolTopFive(rows []GaspolRankingRow) []GaspolRankingRow {
	return selectGaspolTopRows(rows, 5)
}

func selectGaspolTopRows(rows []GaspolRankingRow, limit int) []GaspolRankingRow {
	if limit <= 0 || len(rows) == 0 {
		return []GaspolRankingRow{}
	}
	if len(rows) <= limit {
		return append([]GaspolRankingRow(nil), rows...)
	}
	return append([]GaspolRankingRow(nil), rows[:limit]...)
}
