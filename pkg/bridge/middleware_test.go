package bridge

import (
	"context"
	"testing"
)

func TestChainUsesDeclarationOrder(t *testing.T) {
	var calls []string
	final := HandlerFunc[string](func(context.Context, Event[string]) (Disposition, error) {
		calls = append(calls, "handler")
		return Continue, nil
	})
	wrap := func(name string) Middleware[string] {
		return func(next Handler[string]) Handler[string] {
			return HandlerFunc[string](func(ctx context.Context, event Event[string]) (Disposition, error) {
				calls = append(calls, name+" before")
				result, err := next.Handle(ctx, event)
				calls = append(calls, name+" after")
				return result, err
			})
		}
	}

	if _, err := Chain(final, wrap("one"), wrap("two")).Handle(context.Background(), Event[string]{}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	want := []string{"one before", "two before", "handler", "two after", "one after"}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q", i, calls[i], want[i])
		}
	}
}
