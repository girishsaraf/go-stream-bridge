package runtime

import (
	"testing"
	"time"
)

func TestLoadRejectsIncompleteConfiguration(t *testing.T) {
	_, err := Load([]byte(`{"source":{"type":"test"},"sink":{"type":"test"},"pipeline":{"workers":0}}`))
	if err == nil {
		t.Fatal("Load succeeded")
	}
}

func TestLoadParsesPipeline(t *testing.T) {
	config, err := Load([]byte(`{
		"source": {"type": "test", "config": {}},
		"sink": {"type": "test", "config": {}},
		"pipeline": {
			"workers": 1,
			"buffer": 0,
			"messageTimeout": 1000000000,
			"shutdownTimeout": 1000000000
		}
	}`))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if config.Pipeline.MessageTimeout != time.Second {
		t.Fatalf("message timeout = %s", config.Pipeline.MessageTimeout)
	}
}
