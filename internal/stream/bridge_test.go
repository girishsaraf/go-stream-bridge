package stream

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gostreambridge/internal/config"
	"gostreambridge/internal/dlq"
	"gostreambridge/internal/sink"
	"gostreambridge/internal/source"
)

type fakeSource struct {
	messages    []source.Message
	block       bool
	closeCalled bool
}

func (s *fakeSource) Consume(ctx context.Context, deliver source.Deliver) error {
	for _, message := range s.messages {
		if err := deliver(message); err != nil {
			return err
		}
	}
	if s.block {
		<-ctx.Done()
	}
	return nil
}

func (s *fakeSource) Close() error {
	s.closeCalled = true
	return nil
}

type fakeSink struct {
	writeFn     func(ctx context.Context, message []byte) error
	closeCalled bool
	mu          sync.Mutex
	written     [][]byte
}

func (s *fakeSink) Write(ctx context.Context, message []byte) error {
	s.mu.Lock()
	s.written = append(s.written, message)
	s.mu.Unlock()
	if s.writeFn != nil {
		return s.writeFn(ctx, message)
	}
	return nil
}

func (s *fakeSink) Close() error {
	s.closeCalled = true
	return nil
}

func (s *fakeSink) messages() [][]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][]byte(nil), s.written...)
}

type ackResult struct {
	called bool
	err    error
}

func newAckMessage(value string) (source.Message, *ackResult) {
	result := &ackResult{}
	return source.Message{
		Value:  []byte(value),
		Source: "test",
		Ack: func(err error) error {
			result.called = true
			result.err = err
			return nil
		},
	}, result
}

type fakeDLQ struct {
	mu      sync.Mutex
	records []dlq.Record
	err     error
	closed  bool
}

func (d *fakeDLQ) Write(_ context.Context, record dlq.Record) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.records = append(d.records, record)
	return d.err
}

func (d *fakeDLQ) Close() error {
	d.closed = true
	return nil
}

type blockingDLQ struct {
	started chan struct{}
	closed  bool
}

func (d *blockingDLQ) Write(ctx context.Context, _ dlq.Record) error {
	close(d.started)
	<-ctx.Done()
	return ctx.Err()
}

func (d *blockingDLQ) Close() error {
	d.closed = true
	return nil
}

func testRuntime() config.Bridge {
	cfg := config.DefaultBridge()
	cfg.Buffer = 16
	cfg.MessageTimeout = config.Duration(2 * time.Second)
	cfg.ShutdownTimeout = config.Duration(2 * time.Second)
	cfg.Retry = config.RetryPolicy{
		Attempts:  3,
		BaseDelay: config.Duration(time.Millisecond),
		MaxDelay:  config.Duration(5 * time.Millisecond),
	}
	return cfg
}

func TestRunProcessesMessagesAndAcks(t *testing.T) {
	first, firstAck := newAckMessage("one")
	second, secondAck := newAckMessage("two")
	src := &fakeSource{messages: []source.Message{first, second}}
	dst := &fakeSink{}

	if err := Run(context.Background(), src, dst, testRuntime(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got := dst.messages()
	if len(got) != 2 || string(got[0]) != "one" || string(got[1]) != "two" {
		t.Fatalf("unexpected sink writes: %q", got)
	}
	for _, ack := range []*ackResult{firstAck, secondAck} {
		if !ack.called || ack.err != nil {
			t.Fatalf("expected successful ack, got called=%v err=%v", ack.called, ack.err)
		}
	}
	if !src.closeCalled || !dst.closeCalled {
		t.Error("expected both source and sink to be closed")
	}
}

func TestRunRetriesTransientErrors(t *testing.T) {
	message, ack := newAckMessage("one")
	src := &fakeSource{messages: []source.Message{message}}

	attempts := 0
	dst := &fakeSink{writeFn: func(context.Context, []byte) error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary")
		}
		return nil
	}}

	if err := Run(context.Background(), src, dst, testRuntime(), nil); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
	if !ack.called || ack.err != nil {
		t.Fatalf("expected successful ack, got called=%v err=%v", ack.called, ack.err)
	}
}

func TestRunDoesNotRetryPermanentErrors(t *testing.T) {
	message, ack := newAckMessage("one")
	src := &fakeSource{messages: []source.Message{message}}

	attempts := 0
	dst := &fakeSink{writeFn: func(context.Context, []byte) error {
		attempts++
		return sink.Permanent(errors.New("bad schema"))
	}}

	err := Run(context.Background(), src, dst, testRuntime(), nil)
	if err == nil {
		t.Fatal("expected Run to fail")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt, got %d", attempts)
	}
	if !ack.called || ack.err == nil {
		t.Fatalf("expected failed ack, got called=%v err=%v", ack.called, ack.err)
	}
	if !src.closeCalled || !dst.closeCalled {
		t.Error("expected both source and sink to be closed")
	}
}

func TestRunWritesTerminalFailuresToDLQ(t *testing.T) {
	message, ack := newAckMessage("poison")
	message.Topic = "input"
	message.Partition = 2
	message.Offset = 42
	src := &fakeSource{messages: []source.Message{message}}

	dst := &fakeSink{writeFn: func(context.Context, []byte) error {
		return sink.Permanent(errors.New("invalid document"))
	}}
	writer := &fakeDLQ{}

	cfg := testRuntime()
	cfg.FailurePolicy = config.FailurePolicyDLQ
	cfg.DLQ = config.DLQ{Type: config.DLQTypeFile, Path: "/tmp/unused"}

	if err := Run(context.Background(), src, dst, cfg, writer); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(writer.records) != 1 {
		t.Fatalf("expected 1 DLQ record, got %d", len(writer.records))
	}
	record := writer.records[0]
	if record.Topic != "input" || record.Partition != 2 || record.Offset != 42 {
		t.Errorf("unexpected DLQ metadata: %+v", record)
	}
	if string(record.Payload) != "poison" {
		t.Errorf("unexpected DLQ payload: %q", record.Payload)
	}
	if record.Attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", record.Attempts)
	}
	if !ack.called || ack.err != nil {
		t.Fatalf("expected dead-lettered message to be acked, got called=%v err=%v", ack.called, ack.err)
	}
	if !writer.closed {
		t.Error("expected DLQ writer to be closed")
	}
}

func TestRunStopsWhenDLQWriteFails(t *testing.T) {
	message, ack := newAckMessage("one")
	src := &fakeSource{messages: []source.Message{message}}
	dst := &fakeSink{writeFn: func(context.Context, []byte) error {
		return errors.New("downstream unavailable")
	}}
	writer := &fakeDLQ{err: errors.New("DLQ unavailable")}

	cfg := testRuntime()
	cfg.FailurePolicy = config.FailurePolicyDLQ
	cfg.DLQ = config.DLQ{Type: config.DLQTypeFile, Path: "/tmp/unused"}

	err := Run(context.Background(), src, dst, cfg, writer)
	if err == nil {
		t.Fatal("expected Run to fail when the DLQ is unavailable")
	}
	if !ack.called || ack.err == nil {
		t.Fatalf("expected non-nil ack error, got called=%v err=%v", ack.called, ack.err)
	}
}

func TestRunProcessesConcurrently(t *testing.T) {
	cfg := testRuntime()
	cfg.Workers = 4
	cfg.Buffer = 8

	messages := make([]source.Message, 4)
	acks := make([]*ackResult, 4)
	for i := range messages {
		messages[i], acks[i] = newAckMessage("message")
	}
	src := &fakeSource{messages: messages}

	started := make(chan struct{}, len(messages))
	release := make(chan struct{})
	dst := &fakeSink{writeFn: func(context.Context, []byte) error {
		started <- struct{}{}
		<-release
		return nil
	}}

	done := make(chan error, 1)
	go func() {
		done <- Run(context.Background(), src, dst, cfg, nil)
	}()

	for i := 0; i < len(messages); i++ {
		select {
		case <-started:
		case <-time.After(2 * time.Second):
			close(release)
			t.Fatalf("only %d of %d workers started concurrently", i, len(messages))
		}
	}
	close(release)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not finish")
	}

	for i, ack := range acks {
		if !ack.called || ack.err != nil {
			t.Fatalf("message %d was not acked successfully", i)
		}
	}
}

func TestRunGracefulShutdownLeavesInFlightMessageUnacked(t *testing.T) {
	message, ack := newAckMessage("one")
	src := &fakeSource{messages: []source.Message{message}, block: true}

	started := make(chan struct{})
	dst := &fakeSink{writeFn: func(ctx context.Context, _ []byte) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	}}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, src, dst, testRuntime(), nil)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sink was not called")
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected graceful shutdown, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancellation")
	}

	if ack.called {
		t.Error("in-flight message must not be acknowledged during shutdown")
	}
}

func TestRunGracefulShutdownDuringDLQWriteLeavesMessageUnacked(t *testing.T) {
	message, ack := newAckMessage("poison")
	src := &fakeSource{messages: []source.Message{message}, block: true}
	dst := &fakeSink{writeFn: func(context.Context, []byte) error {
		return sink.Permanent(errors.New("invalid message"))
	}}
	writer := &blockingDLQ{started: make(chan struct{})}

	cfg := testRuntime()
	cfg.FailurePolicy = config.FailurePolicyDLQ
	cfg.DLQ = config.DLQ{Type: config.DLQTypeFile, Path: "/tmp/unused"}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, src, dst, cfg, writer)
	}()

	select {
	case <-writer.started:
	case <-time.After(2 * time.Second):
		cancel()
		t.Fatal("DLQ write was not started")
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected graceful shutdown, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancellation")
	}

	if ack.called {
		t.Error("message must remain unacknowledged when shutdown interrupts its DLQ write")
	}
	if !writer.closed {
		t.Error("expected DLQ writer to be closed after workers stop")
	}
}

func TestRunFailsOnAckError(t *testing.T) {
	ackErr := errors.New("ack failed")
	message, _ := newAckMessage("one")
	message.Ack = func(error) error { return ackErr }

	src := &fakeSource{messages: []source.Message{message}}
	err := Run(context.Background(), src, &fakeSink{}, testRuntime(), nil)
	if !errors.Is(err, ackErr) {
		t.Fatalf("expected ack error, got %v", err)
	}
}

func TestRunAppliesMessageTimeout(t *testing.T) {
	message, _ := newAckMessage("one")
	src := &fakeSource{messages: []source.Message{message}}

	dst := &fakeSink{writeFn: func(ctx context.Context, _ []byte) error {
		<-ctx.Done()
		return ctx.Err()
	}}

	cfg := testRuntime()
	cfg.MessageTimeout = config.Duration(20 * time.Millisecond)

	start := time.Now()
	err := Run(context.Background(), src, dst, cfg, nil)
	if err == nil {
		t.Fatal("expected message timeout to fail the bridge")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("message timeout took too long: %s", elapsed)
	}
}

func TestRunShutdownTimeout(t *testing.T) {
	message, _ := newAckMessage("one")
	src := &fakeSource{messages: []source.Message{message}, block: true}

	started := make(chan struct{})
	release := make(chan struct{})
	dst := &fakeSink{writeFn: func(context.Context, []byte) error {
		close(started)
		<-release
		return nil
	}}

	cfg := testRuntime()
	cfg.ShutdownTimeout = config.Duration(20 * time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, src, dst, cfg, nil)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("sink was not called")
	}
	cancel()

	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "shutdown timed out") {
			t.Fatalf("expected shutdown timeout error, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not respect the shutdown timeout")
	}
	close(release)
}

func TestRunRejectsInvalidRuntimeConfig(t *testing.T) {
	cfg := testRuntime()
	cfg.Workers = 0

	if err := Run(context.Background(), &fakeSource{}, &fakeSink{}, cfg, nil); err == nil {
		t.Fatal("expected invalid config to be rejected")
	}
}

func TestRunRejectsDLQPolicyWithoutWriter(t *testing.T) {
	cfg := testRuntime()
	cfg.FailurePolicy = config.FailurePolicyDLQ
	cfg.DLQ = config.DLQ{Type: config.DLQTypeFile, Path: "/tmp/unused"}

	if err := Run(context.Background(), &fakeSource{}, &fakeSink{}, cfg, nil); err == nil {
		t.Fatal("expected missing DLQ writer to be rejected")
	}
}
