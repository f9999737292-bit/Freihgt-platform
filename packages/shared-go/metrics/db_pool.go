package metrics

import (
	"database/sql"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type poolStatsSnapshot struct {
	openConnections    float64
	inUseConnections float64
	idleConnections  float64
	waitCount        float64
	waitDuration     float64
	maxOpen          float64
}

var registeredPoolMetrics sync.Map

func registerPoolMetrics(serviceName string, snapshot func() poolStatsSnapshot) {
	if _, loaded := registeredPoolMetrics.LoadOrStore(serviceName, true); loaded {
		return
	}

	labels := prometheus.Labels{"service": serviceName}

	prometheus.MustRegister(
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:        "db_pool_open_connections",
			Help:        "Number of established connections in the pool",
			ConstLabels: labels,
		}, func() float64 { return snapshot().openConnections }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:        "db_pool_in_use_connections",
			Help:        "Number of connections currently in use",
			ConstLabels: labels,
		}, func() float64 { return snapshot().inUseConnections }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:        "db_pool_idle_connections",
			Help:        "Number of idle connections in the pool",
			ConstLabels: labels,
		}, func() float64 { return snapshot().idleConnections }),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name:        "db_pool_max_open_connections",
			Help:        "Maximum number of open connections allowed",
			ConstLabels: labels,
		}, func() float64 { return snapshot().maxOpen }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name:        "db_pool_wait_count_total",
			Help:        "Total number of connections waited for",
			ConstLabels: labels,
		}, func() float64 { return snapshot().waitCount }),
		prometheus.NewCounterFunc(prometheus.CounterOpts{
			Name:        "db_pool_wait_duration_seconds_total",
			Help:        "Total time blocked waiting for a new connection",
			ConstLabels: labels,
		}, func() float64 { return snapshot().waitDuration }),
	)
}

// RegisterDBPoolMetrics exposes database/sql pool statistics on /metrics.
func RegisterDBPoolMetrics(serviceName string, db *sql.DB) {
	if db == nil {
		return
	}
	registerPoolMetrics(serviceName, func() poolStatsSnapshot {
		stats := db.Stats()
		return poolStatsSnapshot{
			openConnections:    float64(stats.OpenConnections),
			inUseConnections: float64(stats.InUse),
			idleConnections:  float64(stats.Idle),
			waitCount:        float64(stats.WaitCount),
			waitDuration:     stats.WaitDuration.Seconds(),
			maxOpen:          float64(stats.MaxOpenConnections),
		}
	})
}

// RegisterPgxPoolMetrics exposes pgxpool statistics using the same metric names.
func RegisterPgxPoolMetrics(serviceName string, pool *pgxpool.Pool) {
	if pool == nil {
		return
	}
	registerPoolMetrics(serviceName, func() poolStatsSnapshot {
		stat := pool.Stat()
		return poolStatsSnapshot{
			openConnections:    float64(stat.TotalConns()),
			inUseConnections: float64(stat.AcquiredConns()),
			idleConnections:  float64(stat.IdleConns()),
			waitCount:        float64(stat.EmptyAcquireCount()),
			waitDuration:     stat.AcquireDuration().Seconds(),
			maxOpen:          float64(stat.MaxConns()),
		}
	})
}
