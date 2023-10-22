package internal

import (
	"log"

	promsdk "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"k8s.io/client-go/tools/leaderelection"
)

type leaderElectionMetrics struct {
	isLeader promsdk.Gauge
}

func (m *leaderElectionMetrics) NewLeaderMetric() leaderelection.SwitchMetric {
	return m
}

func (m *leaderElectionMetrics) On(name string) {
	m.isLeader.Set(1)
}

func (m *leaderElectionMetrics) Off(name string) {
	m.isLeader.Set(0)
}

func (a *App) configureMetrics() error {
	exporter, err := prometheus.New()
	if err != nil {
		log.Fatal(err)
	}
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
	otel.SetMeterProvider(provider)

	if err := runtimemetrics.Start(); err != nil {
		return err
	}

	g := promauto.NewGauge(promsdk.GaugeOpts{
		Name: "is_leader",
		Help: "Set to 1 if current instance is leader and 0 if otherwise",
		ConstLabels: promsdk.Labels{
			"pod_name":       a.conf.PodName,
			"member_id":      a.conf.MemberID,
			"election_group": a.conf.ElectionGroup,
			"namespace":      a.conf.Namespace,
		},
	})
	leaderelection.SetProvider(&leaderElectionMetrics{isLeader: g})

	return nil
}
