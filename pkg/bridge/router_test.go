package bridge

import (
	"context"
	"errors"
	"testing"
)

type recordingSink struct{ calls int }

func (s *recordingSink) Open(context.Context) error                   { return nil }
func (s *recordingSink) Close(context.Context) error                  { return nil }
func (s *recordingSink) Deliver(context.Context, Event[string]) error { s.calls++; return nil }

func TestRouterDeliversToEveryRequiredSink(t *testing.T) {
	first, second := &recordingSink{}, &recordingSink{}
	router, err := NewRouter([]Route[string]{{
		Name:     "all",
		Match:    PredicateFunc[string](func(context.Context, Event[string]) (bool, error) { return true, nil }),
		Required: []string{"first", "second"},
	}}, map[string]Sink[string]{"first": first, "second": second})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if _, err := router.Deliver(context.Background(), Event[string]{}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if first.calls != 1 || second.calls != 1 {
		t.Fatalf("sink calls = %d, %d", first.calls, second.calls)
	}
}

func TestRouterRejectsUnmatchedEvents(t *testing.T) {
	router, err := NewRouter([]Route[string]{{
		Name:     "none",
		Match:    PredicateFunc[string](func(context.Context, Event[string]) (bool, error) { return false, nil }),
		Required: []string{"sink"},
	}}, map[string]Sink[string]{"sink": &recordingSink{}})
	if err != nil {
		t.Fatalf("NewRouter: %v", err)
	}
	if _, err := router.Deliver(context.Background(), Event[string]{}); !errors.Is(err, ErrNoRoute) {
		t.Fatalf("Deliver error = %v, want ErrNoRoute", err)
	}
}
