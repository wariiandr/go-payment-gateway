package telemetry

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func InitMetrics() (func(context.Context) error, http.Handler, error) {
	exporter, err := promexporter.New()
	if err != nil {
		return nil, nil, err
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
	)

	otel.SetMeterProvider(mp)

	return mp.Shutdown, promhttp.Handler(), nil
}
