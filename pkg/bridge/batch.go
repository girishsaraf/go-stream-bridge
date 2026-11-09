package bridge

import (
	"context"
	"time"
)

// BatchSink is an optional Sink capability for bulk delivery.
type BatchSink[T any] interface {
	Sink[T]
	DeliverBatch(context.Context, []Event[T]) ([]DeliveryResult, error)
}

// DeliveryResult gives a terminal result for one batch member.
type DeliveryResult struct {
	Index int
	Err   error
}

// BatchPolicy bounds buffered events and the number of active bulk writes.
type BatchPolicy struct {
	MaxEvents          int
	MaxBytes           int
	MaxWait            time.Duration
	MaxInFlightBatches int
}

// Validate reports whether the policy can safely bound a batcher.
func (p BatchPolicy) Validate() error {
	if p.MaxEvents <= 0 {
		return ErrInvalidBatchPolicy("max events must be positive")
	}
	if p.MaxBytes <= 0 {
		return ErrInvalidBatchPolicy("max bytes must be positive")
	}
	if p.MaxWait <= 0 {
		return ErrInvalidBatchPolicy("max wait must be positive")
	}
	if p.MaxInFlightBatches <= 0 {
		return ErrInvalidBatchPolicy("max in-flight batches must be positive")
	}
	return nil
}

// InvalidBatchPolicyError identifies an unsafe batching configuration.
type InvalidBatchPolicyError string

func (e InvalidBatchPolicyError) Error() string { return string(e) }

// ErrInvalidBatchPolicy builds an InvalidBatchPolicyError.
func ErrInvalidBatchPolicy(reason string) error { return InvalidBatchPolicyError(reason) }
