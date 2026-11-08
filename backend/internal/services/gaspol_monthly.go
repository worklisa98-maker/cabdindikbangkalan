package services

// Use the current weighted component model; zero-final is complete, not missing.
func gaspolMonthlySeriesIsFinal(components []GaspolMonthlyComponent) bool {
	if len(components) != 6 {
		return false
	}
	seen := map[int]bool{}
	for _, component := range components {
		if component.Month < 4 || component.Month > 9 || seen[component.Month] || !gaspolMonthlyComponentStatusIsFinal(component.Status) {
			return false
		}
		seen[component.Month] = true
	}
	return true
}
func gaspolTopTenReady(rows []GaspolRankingRow) bool {
	if len(rows) == 0 {
		return false
	}
	for _, row := range rows {
		if !gaspolMonthlySeriesIsFinal(row.MonthlyComponents) {
			return false
		}
	}
	return true
}
func selectGaspolReadyTopTen(rows []GaspolRankingRow) []GaspolRankingRow {
	if !gaspolTopTenReady(rows) {
		return []GaspolRankingRow{}
	}
	return SelectGaspolTopTen(rows)
}
