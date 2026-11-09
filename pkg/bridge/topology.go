package bridge

import "context"

// Topology is an immutable desired-state configuration revision.
type Topology struct {
	Generation uint64
	Pipelines  []PipelineSpec
}

// PipelineSpec is a named pipeline declaration. Connector-specific
// configuration belongs in the registry factory configuration.
type PipelineSpec struct {
	Name string
}

// ApplyResult summarizes a topology reconciliation.
type ApplyResult struct {
	Generation uint64
	Started    []string
	Draining   []string
}

// TopologyManager validates and atomically reconciles topology snapshots.
type TopologyManager interface {
	Apply(context.Context, Topology) (ApplyResult, error)
	Current() Topology
}
