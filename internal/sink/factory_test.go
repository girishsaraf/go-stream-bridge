package sink

import (
	"context"
	"testing"
)

func TestNewRejectsUnsupportedApp(t *testing.T) {
	if _, err := New(context.Background(), "bogus", t.TempDir()); err == nil {
		t.Error("expected an error for an unsupported downstream app")
	}
}
