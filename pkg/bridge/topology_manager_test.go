package bridge

import (
	"context"
	"testing"
)

func TestTopologyManagerPublishesDefensiveSnapshots(t *testing.T) {
	manager := NewTopologyManager(nil)
	topology := Topology{Generation: 1, Pipelines: []PipelineSpec{{Name: "events"}}}
	result, err := manager.Apply(context.Background(), topology)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(result.Started) != 1 || result.Started[0] != "events" {
		t.Fatalf("started = %v", result.Started)
	}
	topology.Pipelines[0].Name = "changed"
	if got := manager.Current().Pipelines[0].Name; got != "events" {
		t.Fatalf("Current pipeline = %q, want events", got)
	}
	if _, err := manager.Apply(context.Background(), Topology{Generation: 1}); err == nil {
		t.Fatal("Apply accepted non-increasing generation")
	}
}
