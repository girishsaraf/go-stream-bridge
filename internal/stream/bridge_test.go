package stream

import (
	"context"
	"errors"
	"testing"

	"gostreambridge/internal/source"
)

type fakeSource struct {
	message     []byte
	closeCalled bool
}

func (s *fakeSource) Consume(ctx context.Context, handle source.Handler) error {
	return handle(ctx, s.message)
}

func (s *fakeSource) Close() error {
	s.closeCalled = true
	return nil
}

type fakeSink struct {
	written     [][]byte
	writeErr    error
	closeCalled bool
}

func (s *fakeSink) Write(_ context.Context, message []byte) error {
	s.written = append(s.written, message)
	return s.writeErr
}

func (s *fakeSink) Close() error {
	s.closeCalled = true
	return nil
}

func TestRunWiresSourceToSink(t *testing.T) {
	src := &fakeSource{message: []byte("hello")}
	dst := &fakeSink{}

	if err := Run(context.Background(), src, dst); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(dst.written) != 1 || string(dst.written[0]) != "hello" {
		t.Fatalf("unexpected sink writes: %q", dst.written)
	}
	if !src.closeCalled || !dst.closeCalled {
		t.Error("expected both source and sink to be closed")
	}
}

func TestRunReturnsWriteErrorAndClosesResources(t *testing.T) {
	writeErr := errors.New("write failed")
	src := &fakeSource{message: []byte("hello")}
	dst := &fakeSink{writeErr: writeErr}

	err := Run(context.Background(), src, dst)
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected write error, got %v", err)
	}
	if !src.closeCalled || !dst.closeCalled {
		t.Error("expected both source and sink to be closed on error")
	}
}
