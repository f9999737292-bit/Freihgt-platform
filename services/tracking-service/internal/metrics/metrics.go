package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Collector struct {
	eventsReceived     prometheus.Counter
	eventsRejected     prometheus.Counter
	eventsDeduplicated prometheus.Counter
	ingestionLag       prometheus.Histogram
	activeShipments    prometheus.Gauge
	staleShipments     prometheus.Gauge
	lostShipments      prometheus.Gauge
}

func New(serviceName string) *Collector {
	c := &Collector{
		eventsReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "tracking_ingest", Name: "telemetry_events_received_total", Help: "Accepted telemetry events",
		}),
		eventsRejected: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "tracking_ingest", Name: "telemetry_events_rejected_total", Help: "Rejected telemetry events",
		}),
		eventsDeduplicated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "tracking_ingest", Name: "telemetry_events_deduplicated_total", Help: "Deduplicated telemetry events",
		}),
		ingestionLag: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "tracking_ingest", Name: "telemetry_ingestion_lag_seconds", Help: "Lag between recordedAt and ingestion",
			Buckets: prometheus.DefBuckets,
		}),
		activeShipments: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tracking_state", Name: "tracking_active_shipments", Help: "Shipments with active tracking",
		}),
		staleShipments: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tracking_state", Name: "tracking_stale_shipments", Help: "Shipments with stale tracking",
		}),
		lostShipments: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "tracking_state", Name: "tracking_lost_shipments", Help: "Shipments with lost tracking",
		}),
	}
	prometheus.MustRegister(
		c.eventsReceived, c.eventsRejected, c.eventsDeduplicated, c.ingestionLag,
		c.activeShipments, c.staleShipments, c.lostShipments,
	)
	return c
}

func (c *Collector) IncReceived()     { c.eventsReceived.Inc() }
func (c *Collector) IncRejected()     { c.eventsRejected.Inc() }
func (c *Collector) IncDeduplicated() { c.eventsDeduplicated.Inc() }
func (c *Collector) ObserveIngestionLag(d interface{ Seconds() float64 }) {
	c.ingestionLag.Observe(d.Seconds())
}
