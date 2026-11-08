package db

import "time"

type PoolStats struct {
	Available            bool
	TotalConns           int32
	AcquiredConns        int32
	IdleConns            int32
	MaxConns             int32
	ConstructingConns    int32
	AcquireCount         int64
	EmptyAcquireCount    int64
	CanceledAcquireCount int64
	AcquireDuration      time.Duration
	EmptyAcquireWaitTime time.Duration
}

func (db *DB) PoolStats() PoolStats {
	if db == nil || db.Pool == nil {
		return PoolStats{}
	}

	stats := db.Pool.Stat()
	return PoolStats{
		Available:            true,
		TotalConns:           stats.TotalConns(),
		AcquiredConns:        stats.AcquiredConns(),
		IdleConns:            stats.IdleConns(),
		MaxConns:             stats.MaxConns(),
		ConstructingConns:    stats.ConstructingConns(),
		AcquireCount:         stats.AcquireCount(),
		EmptyAcquireCount:    stats.EmptyAcquireCount(),
		CanceledAcquireCount: stats.CanceledAcquireCount(),
		AcquireDuration:      stats.AcquireDuration(),
		EmptyAcquireWaitTime: stats.EmptyAcquireWaitTime(),
	}
}
