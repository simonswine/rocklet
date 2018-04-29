package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

const (
	// Namespace is the namespace for rocklet metrics
	Namespace = "rocklet"
)

type Metrics struct {
	logger zerolog.Logger

	registry               *prometheus.Registry
	BatteryLevel           *prometheus.GaugeVec
	AreaCleanedTotal       *prometheus.CounterVec
	TimeSpentCleaningTotal *prometheus.CounterVec
}

func New(logger zerolog.Logger) *Metrics {

	// Create server and register prometheus metrics handler
	s := &Metrics{
		registry: prometheus.NewRegistry(),
		BatteryLevel: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: Namespace,
				Name:      "battery_level",
				Help:      "Percentage of battery charged",
			},
			[]string{"node"},
		),
		AreaCleanedTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "area_cleaned_total",
				Help:      "Area in square meters that has been cleaned",
			},
			[]string{"node"},
		),
		TimeSpentCleaningTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: Namespace,
				Name:      "time_spent_cleaning_total",
				Help:      "Total time spent cleaning",
			},
			[]string{"node"},
		),
	}

	s.registry.MustRegister(s.BatteryLevel)
	s.registry.MustRegister(s.AreaCleanedTotal)
	s.registry.MustRegister(s.TimeSpentCleaningTotal)

	return s
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
