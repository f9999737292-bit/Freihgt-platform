package worker

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var registerOnce sync.Once

type Metrics struct {
	runsTotal      prometheus.Counter
	closedTotal    prometheus.Counter
	errorsTotal    prometheus.Counter
	lastSuccessTS  prometheus.Gauge
}

func NewMetrics(serviceName string) *Metrics {
	m := &Metrics{
		runsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rfx_deadline_worker",
			Name:      "runs_total",
			Help:      "Total deadline worker scan runs",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}),
		closedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rfx_deadline_worker",
			Name:      "closed_total",
			Help:      "Total RFx events auto-closed by deadline worker",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}),
		errorsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "rfx_deadline_worker",
			Name:      "errors_total",
			Help:      "Total deadline worker scan or per-event errors",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}),
		lastSuccessTS: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "rfx_deadline_worker",
			Name:      "last_success_timestamp",
			Help:      "Unix timestamp of last successful deadline worker scan",
			ConstLabels: prometheus.Labels{"service": serviceName},
		}),
	}
	registerOnce.Do(func() {
		prometheus.MustRegister(m.runsTotal, m.closedTotal, m.errorsTotal, m.lastSuccessTS)
	})
	return m
}

func (m *Metrics) IncRuns() { m.runsTotal.Inc() }
func (m *Metrics) AddClosed(n int) {
	if n > 0 {
		m.closedTotal.Add(float64(n))
	}
}
func (m *Metrics) IncErrors() { m.errorsTotal.Inc() }
func (m *Metrics) MarkSuccess(unixTS float64) { m.lastSuccessTS.Set(unixTS) }
