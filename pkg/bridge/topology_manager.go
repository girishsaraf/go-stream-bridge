package bridge

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// TopologyValidator validates a topology before it becomes active.
type TopologyValidator interface {
	Validate(context.Context, Topology) error
}

// TopologyValidatorFunc adapts a function to TopologyValidator.
type TopologyValidatorFunc func(context.Context, Topology) error

// Validate invokes f.
func (f TopologyValidatorFunc) Validate(ctx context.Context, topology Topology) error {
	return f(ctx, topology)
}

// MemoryTopologyManager atomically publishes valid immutable topology
// snapshots. A caller performs connector startup/drain around Apply.
type MemoryTopologyManager struct {
	mu        sync.RWMutex
	current   Topology
	validator TopologyValidator
}

// NewTopologyManager creates a snapshot manager.
func NewTopologyManager(validator TopologyValidator) *MemoryTopologyManager {
	return &MemoryTopologyManager{validator: validator}
}

// Apply validates and publishes a strictly newer topology generation.
func (m *MemoryTopologyManager) Apply(ctx context.Context, topology Topology) (ApplyResult, error) {
	if err := validateTopology(topology); err != nil {
		return ApplyResult{}, err
	}
	if m.validator != nil {
		if err := m.validator.Validate(ctx, cloneTopology(topology)); err != nil {
			return ApplyResult{}, fmt.Errorf("validate topology: %w", err)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if topology.Generation <= m.current.Generation {
		return ApplyResult{}, fmt.Errorf("topology generation %d must be greater than current generation %d", topology.Generation, m.current.Generation)
	}
	old := m.current
	m.current = cloneTopology(topology)
	return ApplyResult{
		Generation: topology.Generation,
		Started:    pipelineNames(topology.Pipelines),
		Draining:   pipelineNames(old.Pipelines),
	}, nil
}

// Current returns a defensive topology snapshot.
func (m *MemoryTopologyManager) Current() Topology {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneTopology(m.current)
}

func validateTopology(topology Topology) error {
	if topology.Generation == 0 {
		return errors.New("topology generation must be positive")
	}
	names := make(map[string]struct{}, len(topology.Pipelines))
	for _, pipeline := range topology.Pipelines {
		if pipeline.Name == "" {
			return errors.New("pipeline name is required")
		}
		if _, exists := names[pipeline.Name]; exists {
			return fmt.Errorf("pipeline %q is duplicated", pipeline.Name)
		}
		names[pipeline.Name] = struct{}{}
	}
	return nil
}

func cloneTopology(topology Topology) Topology {
	clone := topology
	clone.Pipelines = append([]PipelineSpec(nil), topology.Pipelines...)
	return clone
}

func pipelineNames(pipelines []PipelineSpec) []string {
	names := make([]string, len(pipelines))
	for i, pipeline := range pipelines {
		names[i] = pipeline.Name
	}
	return names
}
