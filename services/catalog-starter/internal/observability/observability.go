package observability

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Settings struct {
	Service      string
	OTLPEndpoint string
	SampleRatio  float64
}

var requestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "http_server_request_duration_seconds",
	Help:    "Время ответа HTTP по маршрутам",
	Buckets: prometheus.DefBuckets,
}, []string{"service", "method", "route", "status"})

func init() {
	prometheus.MustRegister(requestDuration)
}

func Mount(r chi.Router, ready func(ctx context.Context) error) {
	// TODO шаг 15: пробы и метрики.
	// Кластеру нужны /health/live и /health/ready (готовность проверяет базу через ready
	// и отвечает 503 с кодом NOT_READY), Prometheus нужен /metrics через promhttp.
	_ = ready
}

func Metrics(service string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			route := chi.RouteContext(r.Context()).RoutePattern()
			if route == "" {
				route = "unmatched"
			}
			requestDuration.WithLabelValues(service, r.Method, route, strconv.Itoa(ww.Status())).Observe(time.Since(start).Seconds())
		})
	}
}

func Sampler(ratio float64) sdktrace.Sampler {
	// TODO шаг 15: сэмплирование трасс по доле из настроек, с уважением к решению родителя.
	_ = ratio
	return sdktrace.NeverSample()
}

func Traced(service string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, service)
	}
}

func Tracing(ctx context.Context, settings Settings) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(ctx, otlptracehttp.WithEndpointURL(settings.OTLPEndpoint))
	if err != nil {
		return nil, err
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(resource.NewSchemaless(attribute.String("service.name", settings.Service))),
		sdktrace.WithSampler(Sampler(settings.SampleRatio)),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return provider.Shutdown, nil
}
