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

	registry     *prometheus.Registry
	BatteryLevel *prometheus.GaugeVec
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
	}

	s.registry.MustRegister(s.BatteryLevel)

	return s
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
