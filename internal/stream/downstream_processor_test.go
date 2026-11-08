package stream

import (
	"context"
	"testing"
)

func TestValidateDownstream(t *testing.T) {
	for _, app := range []string{"mysql", "sqlserver", "elastic", "kafka"} {
		if err := ValidateDownstream(app); err != nil {
			t.Errorf("ValidateDownstream(%q) returned unexpected error: %v", app, err)
		}
	}

	if err := ValidateDownstream("bogus"); err == nil {
		t.Error("ValidateDownstream(\"bogus\") expected an error")
	}
}

func TestStartStreamBridgeRejectsUnsupportedApps(t *testing.T) {
	ctx := context.Background()

	if err := StartStreamBridge(ctx, map[string]string{
		"upstreamApp":   "bogus",
		"downstreamApp": "kafka",
	}); err == nil {
		t.Error("expected unsupported upstream app to fail")
	}

	if err := StartStreamBridge(ctx, map[string]string{
		"upstreamApp":   "kafka",
		"downstreamApp": "bogus",
	}); err == nil {
		t.Error("expected unsupported downstream app to fail")
	}
}
