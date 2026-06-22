// Package telemetry initialise OpenTelemetry (traces + métriques + logs) pour le
// control center, avec export OTLP/gRPC vers un Collector. Tout est désactivé si
// OTEL_EXPORTER_OTLP_ENDPOINT n'est pas défini (dev local sans collector → no-op),
// pour ne jamais bloquer le démarrage.
package telemetry

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Enabled indique si l'export OTLP est actif (endpoint configuré).
func Enabled() bool {
	return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")) != ""
}

// Setup configure les providers OTel globaux et renvoie une fonction d'arrêt propre.
// serviceName identifie le service dans les traces/métriques/logs (ex. "control-center").
// Si OTLP n'est pas configuré, on ne fait rien (shutdown = no-op) et on garde les logs
// standards inchangés.
func Setup(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	if !Enabled() {
		log.Println("[otel] OTEL_EXPORTER_OTLP_ENDPOINT non défini → télémétrie désactivée")
		return func(context.Context) error { return nil }, nil
	}

	res, err := resource.New(ctx,
		resource.WithFromEnv(), // OTEL_RESOURCE_ATTRIBUTES
		resource.WithTelemetrySDK(),
		resource.WithProcess(),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(version()),
		),
	)
	if err != nil {
		// resource.New peut renvoyer une erreur "partielle" : on continue avec ce qu'on a.
		res = resource.Default()
	}

	var shutdowns []func(context.Context) error
	shutdown := func(ctx context.Context) error {
		var errs []error
		for i := len(shutdowns) - 1; i >= 0; i-- {
			if e := shutdowns[i](ctx); e != nil {
				errs = append(errs, e)
			}
		}
		return errors.Join(errs...)
	}

	// Propagation W3C TraceContext + Baggage (corrélation HTTP ↔ gRPC).
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	// --- Traces ---
	traceExp, err := otlptracegrpc.New(ctx)
	if err != nil {
		_ = shutdown(ctx)
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	shutdowns = append(shutdowns, tp.Shutdown)

	// --- Métriques ---
	metricExp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		_ = shutdown(ctx)
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp)),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	shutdowns = append(shutdowns, mp.Shutdown)

	// Métriques runtime Go (GC, goroutines, mémoire…).
	if err := runtime.Start(runtime.WithMeterProvider(mp), runtime.WithMinimumReadMemStatsInterval(15*time.Second)); err != nil {
		log.Printf("[otel] runtime metrics: %v", err)
	}

	// --- Logs ---
	logExp, err := otlploggrpc.New(ctx)
	if err != nil {
		_ = shutdown(ctx)
		return nil, err
	}
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExp)),
		sdklog.WithResource(res),
	)
	shutdowns = append(shutdowns, lp.Shutdown)

	// slog par défaut = bridge OTLP + sortie texte sur stderr (conservée pour les
	// fichiers .devlogs → Promtail). Le bridge attache trace_id/span_id quand le log
	// porte un contexte (slog.*Context).
	otelHandler := otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(lp))
	textHandler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(newFanoutHandler(textHandler, otelHandler)))

	// Le package log standard (log.Printf des handlers existants) est redirigé vers
	// slog → on conserve la sortie fichier ET on exporte les logs en OTLP.
	log.SetOutput(slogWriter{})
	log.SetFlags(0) // slog ajoute déjà l'horodatage

	log.Printf("[otel] télémétrie active (service=%s, endpoint=%s)", serviceName, os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	return shutdown, nil
}

func version() string {
	if v := strings.TrimSpace(os.Getenv("SERVICE_VERSION")); v != "" {
		return v
	}
	return "dev"
}

// slogWriter adapte le package log standard vers slog (chaque ligne → un log Info).
type slogWriter struct{}

func (slogWriter) Write(p []byte) (int, error) {
	msg := strings.TrimRight(string(p), "\n")
	slog.Default().Log(context.Background(), slog.LevelInfo, msg)
	return len(p), nil
}

// fanoutHandler diffuse chaque enregistrement slog vers plusieurs handlers.
type fanoutHandler struct{ handlers []slog.Handler }

func newFanoutHandler(h ...slog.Handler) *fanoutHandler { return &fanoutHandler{handlers: h} }

func (f *fanoutHandler) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f *fanoutHandler) Handle(ctx context.Context, r slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, r.Level) {
			if e := h.Handle(ctx, r.Clone()); e != nil {
				errs = append(errs, e)
			}
		}
	}
	return errors.Join(errs...)
}

func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: next}
}

func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: next}
}

var _ slog.Handler = (*fanoutHandler)(nil)
var _ io.Writer = slogWriter{}
