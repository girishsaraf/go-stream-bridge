//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"gostreambridge/internal/config"
	"gostreambridge/internal/stream"
)

var nameSequence atomic.Uint64

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), nameSequence.Add(1))
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Skipf("%s is not set; skipping integration test", name)
	}
	return value
}

func envOrDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func writeConfigDir(t *testing.T, files map[string]any) string {
	t.Helper()

	dir := t.TempDir()
	for name, value := range files {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatalf("marshal %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func kafkaConsumerConfig(brokers, topic, group string) map[string]any {
	return map[string]any{"brokers": brokers, "topic": topic, "groupId": group}
}

func kafkaProducerConfig(brokers, topic string) map[string]any {
	return map[string]any{"brokers": brokers, "topic": topic}
}

func mysqlConfig(host, port, user, password, database string) map[string]any {
	return map[string]any{
		"host":     host,
		"port":     port,
		"username": user,
		"password": password,
		"database": database,
	}
}

func elasticConfig(url, index string) map[string]any {
	return map[string]any{"url": url, "index": index}
}

func rabbitConfig(url, queue string) map[string]any {
	return map[string]any{
		"url":      url,
		"queue":    queue,
		"prefetch": 20,
		"durable":  true,
	}
}

func bridgeConfig(workers int, failurePolicy string, dlq map[string]any) map[string]any {
	cfg := map[string]any{
		"workers":         workers,
		"buffer":          128,
		"messageTimeout":  "15s",
		"shutdownTimeout": "10s",
		"failurePolicy":   failurePolicy,
		"retry": map[string]any{
			"attempts":  3,
			"baseDelay": "100ms",
			"maxDelay":  "1s",
		},
	}
	if dlq != nil {
		cfg["dlq"] = dlq
	}
	return cfg
}

type bridgeRun struct {
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func startBridge(t *testing.T, configDir, upstream, downstream string) *bridgeRun {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	run := &bridgeRun{cancel: cancel, done: make(chan struct{})}
	go func() {
		run.err = stream.StartStreamBridge(ctx, stream.Options{
			UpstreamApp:   upstream,
			DownstreamApp: downstream,
			ConfigDir:     configDir,
		})
		close(run.done)
	}()
	t.Cleanup(func() { _ = run.stop(t) })
	return run
}

func (b *bridgeRun) wait(t *testing.T, timeout time.Duration) error {
	t.Helper()
	select {
	case <-b.done:
		return b.err
	case <-time.After(timeout):
		t.Fatalf("bridge did not stop within %s", timeout)
		return nil
	}
}

func (b *bridgeRun) stop(t *testing.T) error {
	t.Helper()
	b.cancel()
	return b.wait(t, 20*time.Second)
}

func waitFor(t *testing.T, timeout, interval time.Duration, description string, check func() (bool, string)) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	var last string
	for time.Now().Before(deadline) {
		ok, detail := check()
		if ok {
			return
		}
		last = detail
		time.Sleep(interval)
	}
	t.Fatalf("%s: timed out after %s: %s", description, timeout, last)
}

func sortMessages(messages []string) []string {
	sorted := append([]string(nil), messages...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j] < sorted[j-1]; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}
	return sorted
}

func requireSameMessages(t *testing.T, want, got []string) {
	t.Helper()
	want = sortMessages(want)
	got = sortMessages(got)
	if len(want) != len(got) {
		t.Fatalf("expected %d messages, got %d\nwant: %v\ngot:  %v", len(want), len(got), want, got)
	}
	for i := range want {
		if want[i] != got[i] {
			t.Fatalf("message mismatch at %d: want %q, got %q", i, want[i], got[i])
		}
	}
}

func stopPolicy() string { return config.FailurePolicyStop }
func dlqPolicy() string  { return config.FailurePolicyDLQ }
