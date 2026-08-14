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
	etaReceived        prometheus.Counter
	etaRejected        prometheus.Counter
	etaDeduplicated    prometheus.Counter
	etaIngestionLag    prometheus.Histogram
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
		etaReceived: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "eta_ingest", Name: "eta_observations_received_total", Help: "Accepted ETA observations",
		}),
		etaRejected: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "eta_ingest", Name: "eta_observations_rejected_total", Help: "Rejected ETA observations",
		}),
		etaDeduplicated: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "eta_ingest", Name: "eta_observations_deduplicated_total", Help: "Deduplicated ETA observations",
		}),
		etaIngestionLag: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "eta_ingest", Name: "eta_ingestion_lag_seconds", Help: "Lag between sourceObservedAt and ingestion",
			Buckets: prometheus.DefBuckets,
		}),
	}
	prometheus.MustRegister(
		c.eventsReceived, c.eventsRejected, c.eventsDeduplicated, c.ingestionLag,
		c.activeShipments, c.staleShipments, c.lostShipments,
		c.etaReceived, c.etaRejected, c.etaDeduplicated, c.etaIngestionLag,
	)
	return c
}

func (c *Collector) IncReceived()     { c.eventsReceived.Inc() }
func (c *Collector) IncRejected()     { c.eventsRejected.Inc() }
func (c *Collector) IncDeduplicated() { c.eventsDeduplicated.Inc() }
func (c *Collector) ObserveIngestionLag(d interface{ Seconds() float64 }) {
	c.ingestionLag.Observe(d.Seconds())
}

func (c *Collector) IncETAReceived()     { c.etaReceived.Inc() }
func (c *Collector) IncETARejected()     { c.etaRejected.Inc() }
func (c *Collector) IncETADeduplicated() { c.etaDeduplicated.Inc() }
func (c *Collector) ObserveETAIngestionLag(d interface{ Seconds() float64 }) {
	c.etaIngestionLag.Observe(d.Seconds())
}
