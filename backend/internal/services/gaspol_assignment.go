package services

import (
	"math/rand"
	"sort"
	"sync/atomic"
	"time"
)

var gaspolAssignmentSeedCounter int64

type GaspolAssignmentCandidate struct {
	SchoolID             int
	SchoolName           string
	PreviousSupervisorID *int
}

type GaspolSupervisorCandidate struct {
	UserID            int
	DisplayName       string
	CurrentLoad       int
	HistoricalLoad    int
	PreviousMonthLoad int
}

type GaspolAssignmentPlan struct {
	SchoolID               int    `json:"schoolId"`
	SupervisorUserID       *int   `json:"supervisorUserId"`
	SupervisorNameSnapshot string `json:"supervisorNameSnapshot"`
	Method                 string `json:"method"`
}

func BuildGaspolRandomAssignments(year int, month int, schools []GaspolAssignmentCandidate, supervisors []GaspolSupervisorCandidate) []GaspolAssignmentPlan {
	if len(schools) == 0 {
		return []GaspolAssignmentPlan{}
	}

	seed := int64(year*100 + month)
	seed += time.Now().UnixNano() + atomic.AddInt64(&gaspolAssignmentSeedCounter, 1)*7919
	randomizer := rand.New(rand.NewSource(seed))

	shuffled := append([]GaspolAssignmentCandidate(nil), schools...)
	randomizer.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	pool := append([]GaspolSupervisorCandidate(nil), supervisors...)
	randomizer.Shuffle(len(pool), func(i, j int) {
		pool[i], pool[j] = pool[j], pool[i]
	})
	plans := make([]GaspolAssignmentPlan, 0, len(shuffled))
	if len(pool) == 0 {
		for _, school := range shuffled {
			plans = append(plans, GaspolAssignmentPlan{
				SchoolID:               school.SchoolID,
				SupervisorUserID:       nil,
				SupervisorNameSnapshot: "BELUM_DITENTUKAN",
				Method:                 GaspolAssignmentUnassigned,
			})
		}
		return plans
	}

	for _, school := range shuffled {
		sort.SliceStable(pool, func(i, j int) bool {
			if pool[i].CurrentLoad != pool[j].CurrentLoad {
				return pool[i].CurrentLoad < pool[j].CurrentLoad
			}
			if pool[i].HistoricalLoad != pool[j].HistoricalLoad {
				return pool[i].HistoricalLoad < pool[j].HistoricalLoad
			}
			if pool[i].PreviousMonthLoad != pool[j].PreviousMonthLoad {
				return pool[i].PreviousMonthLoad < pool[j].PreviousMonthLoad
			}
			return false
		})

		minCurrent := pool[0].CurrentLoad
		equallyLoaded := make([]int, 0, len(pool))
		for index := range pool {
			if pool[index].CurrentLoad != minCurrent {
				break
			}
			equallyLoaded = append(equallyLoaded, index)
		}
		chosenIndex := -1
		for _, index := range equallyLoaded {
			if school.PreviousSupervisorID != nil && pool[index].UserID == *school.PreviousSupervisorID {
				continue
			}
			chosenIndex = index
			break
		}
		method := GaspolAssignmentRandom
		if chosenIndex < 0 {
			chosenIndex = 0
			method = GaspolAssignmentRandomFallback
		}

		supervisor := pool[chosenIndex]
		supervisor.CurrentLoad++
		pool[chosenIndex] = supervisor

		plans = append(plans, GaspolAssignmentPlan{
			SchoolID:               school.SchoolID,
			SupervisorUserID:       gaspolIntPtr(supervisor.UserID),
			SupervisorNameSnapshot: supervisor.DisplayName,
			Method:                 method,
		})
	}

	return plans
}

func gaspolIntPtr(value int) *int {
	return &value
}
