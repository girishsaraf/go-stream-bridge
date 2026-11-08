package sink

import (
	"context"
	"errors"
	"testing"
)

func TestPermanent(t *testing.T) {
	if Permanent(nil) != nil {
		t.Fatal("Permanent(nil) must stay nil")
	}

	base := errors.New("bad input")
	wrapped := Permanent(base)
	if !IsPermanent(wrapped) {
		t.Fatal("expected permanent error")
	}
	if !errors.Is(wrapped, base) {
		t.Fatal("permanent error must unwrap to its cause")
	}

	again := Permanent(wrapped)
	if !errors.Is(again, base) {
		t.Fatal("double wrapping must preserve the cause")
	}

	if IsPermanent(base) {
		t.Fatal("unwrapped error must not be permanent")
	}
	if IsPermanent(nil) {
		t.Fatal("nil must not be permanent")
	}
}

func TestPermanentContextErrorsRemainRetryable(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		if IsPermanent(classifySQLError("mysql", err)) {
			t.Errorf("%v must not be classified as permanent", err)
		}
	}
}
