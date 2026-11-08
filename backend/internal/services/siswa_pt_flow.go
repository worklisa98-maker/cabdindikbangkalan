package services

import (
	"strings"
	"time"
)

const (
	OutcomeSourcePTAcceptedPending   = "PT_ACCEPTED_PENDING"
	OutcomeProgressStateFinal        = "FINAL"
	OutcomeProgressStatePendingPT    = "PT_ACCEPTED_PENDING"
	OutcomeProgressStatePendingMulti = "PT_ACCEPTED_MULTI_JALUR"

	pendingValidationBucketBelumRencana = "belum_rencana"
	pendingValidationBucketFinalkanPT   = "finalkan_pilihan_siswa"
	pendingValidationBucketMultiJalur   = "multi_jalur"
)

func classifyPendingValidationBucket(isFinal bool, hasInput bool, ptOfferCount int) string {
	if isFinal {
		return ""
	}
	if !hasInput {
		return pendingValidationBucketBelumRencana
	}
	if ptOfferCount > 1 {
		return pendingValidationBucketMultiJalur
	}
	return pendingValidationBucketFinalkanPT
}

func isPTFlowFinalized(finalizedAt *time.Time) bool {
	return finalizedAt != nil
}

func ptIdentitySQL(alias string) string {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		alias = "c"
	}
	return "COALESCE(NULLIF(TRIM(COALESCE(" + alias + ".nisn, '')), ''), 'NAME:' || LOWER(REGEXP_REPLACE(TRIM(COALESCE(" + alias + ".nama_siswa, '')), '\\s+', ' ', 'g')))"
}

func buildPendingValidationResponse(
	belumRencana []SiswaPendingValidationItem,
	finalkanPilihan []SiswaPendingValidationItem,
	multiJalur []SiswaPendingValidationItem,
) SiswaPendingValidationResponse {
	return SiswaPendingValidationResponse{
		BelumRencana:         belumRencana,
		FinalkanPilihanSiswa: finalkanPilihan,
		MultiJalur:           multiJalur,
		BelumFinal:           finalkanPilihan,
		DiterimaMultiJalur:   multiJalur,
	}
}

func selectSingleAcceptedPTOffer(
	offers []SiswaPendingValidationPTOffer,
) (OutcomePTDetail, string, string) {
	if len(offers) == 0 {
		return OutcomePTDetail{}, "", "MISSING_REQUIRED_DATA"
	}
	if len(offers) > 1 {
		return OutcomePTDetail{}, "", "MULTIPLE_PT_OPTIONS"
	}

	offer := offers[0]
	jalur, ok := normalizeJalurPenerimaan(offer.Jalur)
	if !ok {
		return OutcomePTDetail{}, "", "MISSING_REQUIRED_DATA"
	}
	kampus := strings.TrimSpace(offer.Kampus)
	prodi := strings.TrimSpace(offer.Prodi)
	if kampus == "" || prodi == "" {
		return OutcomePTDetail{}, "", "MISSING_REQUIRED_DATA"
	}

	return OutcomePTDetail{
		Kampus:        kampus,
		Prodi:         prodi,
		Jalur:         jalur,
		TahunDiterima: offer.TahunDiterima,
	}, jalur, ""
}

func selectPTFinalizationCandidate(
	offers []SiswaPendingValidationPTOffer,
	registeredJalur []string,
	ptOptions []SiswaPendingValidationPTChoice,
) (OutcomePTDetail, string, string) {
	if detail, jalur, reason := selectSingleAcceptedPTOffer(offers); reason == "" || reason == "MULTIPLE_PT_OPTIONS" {
		return detail, jalur, reason
	}

	if len(registeredJalur) == 0 {
		return OutcomePTDetail{}, "", "MISSING_REQUIRED_DATA"
	}
	if len(registeredJalur) > 1 {
		return OutcomePTDetail{}, "", "MULTIPLE_PT_OPTIONS"
	}

	selectedJalur, validSelectedJalur := normalizeJalurPenerimaan(registeredJalur[0])
	if !validSelectedJalur {
		return OutcomePTDetail{}, "", "MISSING_REQUIRED_DATA"
	}

	validOptions := make([]SiswaPendingValidationPTChoice, 0, len(ptOptions))
	for _, option := range ptOptions {
		normalizedJalur, validJalur := normalizeJalurPenerimaan(option.Jalur)
		if !validJalur || normalizedJalur != selectedJalur {
			continue
		}
		kampus := strings.TrimSpace(option.Kampus)
		prodi := strings.TrimSpace(option.Prodi)
		if kampus == "" || prodi == "" {
			continue
		}
		option.Jalur = normalizedJalur
		validOptions = append(validOptions, option)
	}
	if len(validOptions) != 1 {
		if len(validOptions) > 1 {
			return OutcomePTDetail{}, "", "MULTIPLE_PT_OPTIONS"
		}
		return OutcomePTDetail{}, "", "MISSING_REQUIRED_DATA"
	}

	chosen := validOptions[0]
	return OutcomePTDetail{
		Kampus: strings.TrimSpace(chosen.Kampus),
		Prodi:  strings.TrimSpace(chosen.Prodi),
		Jalur:  selectedJalur,
	}, selectedJalur, ""
}

func autoFinalizeDeadlineForGraduationYear(tahunLulus int, loc *time.Location) time.Time {
	location := loc
	if location == nil {
		location = time.Local
	}
	return time.Date(tahunLulus, time.November, 1, 0, 0, 0, 0, location)
}
