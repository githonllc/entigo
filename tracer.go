package entigo

import "context"

// Span represents a unit of work that can be finished to record its duration.
type Span interface {
	Finish()
}

// Tracer creates spans for tracking operations.
// Implementations can integrate with distributed tracing systems.
type Tracer interface {
	StartSpan(ctx context.Context, operation string) Span
}

// noopSpan is a no-operation span that does nothing on Finish.
type noopSpan struct{}

func (noopSpan) Finish() {}

// NoopTracer is a tracer that produces no-operation spans.
// It is useful as a default when no tracing backend is configured.
type NoopTracer struct{}

// StartSpan returns a no-operation span.
func (NoopTracer) StartSpan(ctx context.Context, operation string) Span {
	return noopSpan{}
}
