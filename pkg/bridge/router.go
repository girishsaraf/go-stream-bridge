package bridge

import (
	"context"
	"errors"
	"fmt"
)

// ErrNoRoute is returned when no configured route accepts an event.
var ErrNoRoute = errors.New("no route matched event")

// Router applies ordered route predicates and delivers matching events to their
// required sinks. Sink lifecycle remains owned by the pipeline runtime.
type Router[T any] struct {
	routes []Route[T]
	sinks  map[string]Sink[T]
}

// NewRouter validates routes against a named sink set.
func NewRouter[T any](routes []Route[T], sinks map[string]Sink[T]) (*Router[T], error) {
	if len(routes) == 0 {
		return nil, errors.New("at least one route is required")
	}
	copySinks := make(map[string]Sink[T], len(sinks))
	for name, sink := range sinks {
		if name == "" || sink == nil {
			return nil, errors.New("router sink name and value are required")
		}
		copySinks[name] = sink
	}
	copyRoutes := append([]Route[T](nil), routes...)
	for _, route := range copyRoutes {
		if route.Name == "" || route.Match == nil {
			return nil, errors.New("route name and predicate are required")
		}
		if len(route.Required) == 0 {
			return nil, fmt.Errorf("route %q has no required sinks", route.Name)
		}
		for _, sinkName := range route.Required {
			if copySinks[sinkName] == nil {
				return nil, fmt.Errorf("route %q references unknown sink %q", route.Name, sinkName)
			}
		}
	}
	return &Router[T]{routes: copyRoutes, sinks: copySinks}, nil
}

// Deliver runs every matching route. It returns Drop only when no route
// matched and a route explicitly configured drop-on-no-match.
func (r *Router[T]) Deliver(ctx context.Context, event Event[T]) (Disposition, error) {
	matched := false
	dropOnNoMatch := false
	for _, route := range r.routes {
		match, err := route.Match.Match(ctx, event)
		if err != nil {
			return Continue, fmt.Errorf("match route %q: %w", route.Name, err)
		}
		if !match {
			dropOnNoMatch = dropOnNoMatch || route.OnNoMatch == Drop
			continue
		}
		matched = true
		final := HandlerFunc[T](func(ctx context.Context, event Event[T]) (Disposition, error) {
			for _, sinkName := range route.Required {
				if err := r.sinks[sinkName].Deliver(ctx, event); err != nil {
					return Continue, fmt.Errorf("deliver route %q to sink %q: %w", route.Name, sinkName, err)
				}
			}
			return Continue, nil
		})
		disposition, err := Chain(final, route.Middleware...).Handle(ctx, event)
		if err != nil || disposition == Drop {
			return disposition, err
		}
	}
	if matched {
		return Continue, nil
	}
	if dropOnNoMatch {
		return Drop, nil
	}
	return Continue, ErrNoRoute
}
