package observability

import (
	"context"
	"testing"
)

func TestTraceIDContext(t *testing.T) {
	ctx := context.Background()
	if got := TraceIDFromContext(ctx); got != "" {
		t.Errorf("expected empty trace ID from background context, got %s", got)
	}

	ctx = ContextWithTraceID(ctx, "abc-123")
	if got := TraceIDFromContext(ctx); got != "abc-123" {
		t.Errorf("expected abc-123, got %s", got)
	}
}
