package bridge

import "context"

// Disposition describes a successful middleware outcome.
type Disposition uint8

const (
	// Continue passes the event to the next pipeline stage.
	Continue Disposition = iota
	// Drop intentionally completes an event without a sink delivery.
	Drop
)

// Handler processes one event.
type Handler[T any] interface {
	Handle(context.Context, Event[T]) (Disposition, error)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc[T any] func(context.Context, Event[T]) (Disposition, error)

// Handle invokes f.
func (f HandlerFunc[T]) Handle(ctx context.Context, event Event[T]) (Disposition, error) {
	return f(ctx, event)
}

// Middleware wraps a handler. Implementations must invoke next at most once.
type Middleware[T any] func(Handler[T]) Handler[T]

// Chain applies middleware in declaration order.
func Chain[T any](final Handler[T], middleware ...Middleware[T]) Handler[T] {
	for i := len(middleware) - 1; i >= 0; i-- {
		final = middleware[i](final)
	}
	return final
}

// Predicate selects events for a route.
type Predicate[T any] interface {
	Match(context.Context, Event[T]) (bool, error)
}

// PredicateFunc adapts a function to Predicate.
type PredicateFunc[T any] func(context.Context, Event[T]) (bool, error)

// Match invokes f.
func (f PredicateFunc[T]) Match(ctx context.Context, event Event[T]) (bool, error) {
	return f(ctx, event)
}

// Route describes an ordered routing decision. Required identifies sinks that
// must complete before the source delivery can be acknowledged.
type Route[T any] struct {
	Name       string
	Match      Predicate[T]
	Required   []string
	Middleware []Middleware[T]
	OnNoMatch  Disposition
}
