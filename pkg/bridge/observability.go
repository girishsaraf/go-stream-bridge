package bridge

import "context"

// Dependencies are the infrastructure services made available to connector
// factories. Every field may be nil; callers should use Defaults first.
type Dependencies struct {
	Logger Logger
	Meter  Meter
	Tracer Tracer
}

// Defaults replaces nil observability dependencies with no-op implementations.
func (d Dependencies) Defaults() Dependencies {
	if d.Logger == nil {
		d.Logger = nopLogger{}
	}
	if d.Meter == nil {
		d.Meter = nopMeter{}
	}
	if d.Tracer == nil {
		d.Tracer = nopTracer{}
	}
	return d
}

// Field is a structured logging field.
type Field struct {
	Key   string
	Value any
}

// Label is a low-cardinality metric dimension.
type Label struct {
	Key   string
	Value string
}

// Logger records structured pipeline events.
type Logger interface {
	With(...Field) Logger
	Debug(context.Context, string, ...Field)
	Info(context.Context, string, ...Field)
	Warn(context.Context, string, ...Field)
	Error(context.Context, error, string, ...Field)
}

// Counter is a monotonically increasing metric.
type Counter interface{ Add(context.Context, int64) }

// Histogram records a distribution value.
type Histogram interface {
	Record(context.Context, float64)
}

// Gauge records a current value.
type Gauge interface {
	Set(context.Context, float64)
}

// Meter creates named metric instruments.
type Meter interface {
	Counter(string, ...Label) Counter
	Histogram(string, ...Label) Histogram
	Gauge(string, ...Label) Gauge
}

// Span is an in-progress tracing span.
type Span interface {
	SetError(error)
	End()
}

// SpanOption configures a tracing span.
type SpanOption interface{ applySpanOption() }

// Tracer creates spans while preserving the incoming context.
type Tracer interface {
	Start(context.Context, string, ...SpanOption) (context.Context, Span)
}

type nopLogger struct{}

func (nopLogger) With(...Field) Logger                           { return nopLogger{} }
func (nopLogger) Debug(context.Context, string, ...Field)        {}
func (nopLogger) Info(context.Context, string, ...Field)         {}
func (nopLogger) Warn(context.Context, string, ...Field)         {}
func (nopLogger) Error(context.Context, error, string, ...Field) {}

type nopMetric struct{}

func (nopMetric) Add(context.Context, int64)      {}
func (nopMetric) Record(context.Context, float64) {}
func (nopMetric) Set(context.Context, float64)    {}

type nopMeter struct{}

func (nopMeter) Counter(string, ...Label) Counter     { return nopMetric{} }
func (nopMeter) Histogram(string, ...Label) Histogram { return nopMetric{} }
func (nopMeter) Gauge(string, ...Label) Gauge         { return nopMetric{} }

type nopSpan struct{}

func (nopSpan) SetError(error) {}
func (nopSpan) End()           {}

type nopTracer struct{}

func (nopTracer) Start(ctx context.Context, _ string, _ ...SpanOption) (context.Context, Span) {
	return ctx, nopSpan{}
}
