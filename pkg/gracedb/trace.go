package gracedb

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("gracedb")
var meter = otel.Meter("gracedb")
var upsertCounter metric.Int64Counter
var searchDuration metric.Float64Histogram

func init() {
	var err error
	upsertCounter, err = meter.Int64Counter("gracedb.upsert.count",
		metric.WithDescription("Total number of upserts"),
	)
	if err != nil {
		return
	}
	searchDuration, err = meter.Float64Histogram("gracedb.search.duration",
		metric.WithDescription("Search latency in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return
	}
}

func spanWithCollection(ctx context.Context, operation, collection string) context.Context {
	ctx, span := tracer.Start(ctx, "gracedb."+operation)
	span.SetAttributes(attribute.String("collection", collection))
	return ctx
}

func endSpan(ctx context.Context, err error) {
	if err != nil {
		span := trace.SpanFromContext(ctx)
		if span.IsRecording() {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
		}
	}
}

func recordSearchDuration(ctx context.Context, start time.Time) {
	if searchDuration == nil {
		return
	}
	elapsed := float64(time.Since(start).Nanoseconds()) / 1e6
	searchDuration.Record(ctx, elapsed)
}

func recordUpsert(ctx context.Context, count int) {
	if upsertCounter == nil {
		return
	}
	upsertCounter.Add(ctx, int64(count))
}

// TraceInfo returns a human-readable summary of telemetry state.
func (db *DB) TraceInfo() string {
	return fmt.Sprintf("tracer=%s, meter=%s", tracer, meter)
}
