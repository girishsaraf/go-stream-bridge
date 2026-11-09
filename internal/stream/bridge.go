// Package stream wires an upstream source to a downstream sink.
package stream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog/log"

	"gostreambridge/internal/config"
	"gostreambridge/internal/dlq"
	"gostreambridge/internal/retry"
	"gostreambridge/internal/sink"
	"gostreambridge/internal/source"
)

// Options describes the bridge selected on the command line.
type Options struct {
	UpstreamApp   string
	DownstreamApp string
	ConfigDir     string
}

// StartStreamBridge constructs the configured source, sink, and dead-letter
// writer, then runs the bridge until the context is cancelled or a component
// fails.
func StartStreamBridge(ctx context.Context, opts Options) error {
	log.Info().
		Str("upstream", opts.UpstreamApp).
		Str("downstream", opts.DownstreamApp).
		Str("configDir", opts.ConfigDir).
		Msg("initializing stream bridge")

	runtimeCfg, err := config.LoadBridge(opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("load runtime config: %w", err)
	}

	src, err := source.New(ctx, opts.UpstreamApp, opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("create upstream source: %w", err)
	}

	dst, err := sink.New(ctx, opts.DownstreamApp, opts.ConfigDir)
	if err != nil {
		return errors.Join(
			fmt.Errorf("create downstream sink: %w", err),
			wrapClose("close upstream source", src.Close()),
		)
	}

	dlqWriter, err := dlq.New(runtimeCfg.DLQ)
	if err != nil {
		return errors.Join(
			fmt.Errorf("create dead-letter writer: %w", err),
			wrapClose("close upstream source", src.Close()),
			wrapClose("close downstream sink", dst.Close()),
		)
	}

	return Run(ctx, src, dst, runtimeCfg, dlqWriter)
}

// Run consumes from src and writes to dst. A fixed worker pool processes
// messages; the source is acknowledged only after a terminal outcome. All
// components are closed before Run returns.
func Run(ctx context.Context, src source.Source, dst sink.Sink, cfg config.Bridge, dlqWriter dlq.Writer) (err error) {
	validationErr := cfg.Validate()
	if validationErr != nil {
		return fmt.Errorf("invalid runtime config: %w", validationErr)
	}
	if cfg.FailurePolicy == config.FailurePolicyDLQ && dlqWriter == nil {
		return errors.New("failure policy dlq requires a dead-letter writer")
	}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	p := &pipeline{
		sink:   dst,
		dlq:    dlqWriter,
		cfg:    cfg,
		retry:  retryPolicy(cfg.Retry),
		runCtx: runCtx,
		cancel: cancel,
	}

	jobs := make(chan source.Message, cfg.Buffer)
	var workers sync.WaitGroup
	for id := 0; id < cfg.Workers; id++ {
		workers.Add(1)
		go func(workerID int) {
			defer workers.Done()
			p.work(runCtx, workerID, jobs)
		}(id)
	}

	log.Info().
		Int("workers", cfg.Workers).
		Int("buffer", cfg.Buffer).
		Dur("messageTimeout", cfg.MessageTimeout.Std()).
		Dur("shutdownTimeout", cfg.ShutdownTimeout.Std()).
		Str("failurePolicy", cfg.FailurePolicy).
		Msg("stream bridge started")

	consumeErr := src.Consume(runCtx, func(message source.Message) error {
		if runCtx.Err() != nil {
			return runCtx.Err()
		}
		select {
		case jobs <- message:
			return nil
		case <-runCtx.Done():
			return runCtx.Err()
		}
	})
	close(jobs)

	waitDone := make(chan struct{})
	go func() {
		workers.Wait()
		close(waitDone)
	}()

	timer := time.NewTimer(cfg.ShutdownTimeout.Std())
	defer timer.Stop()

	select {
	case <-waitDone:
		err = errors.Join(withoutCancellation(consumeErr), p.failure())
		err = errors.Join(
			err,
			wrapClose("close upstream source", src.Close()),
			wrapClose("close downstream sink", dst.Close()),
			wrapClose("close dead-letter writer", closeWriter(dlqWriter)),
		)
	case <-timer.C:
		// A worker ignored cancellation. Leave the sink and DLQ open so we
		// never close resources another goroutine may still be using.
		err = errors.Join(
			withoutCancellation(consumeErr),
			p.failure(),
			fmt.Errorf("shutdown timed out after %s waiting for in-flight messages", cfg.ShutdownTimeout.Std()),
			wrapClose("close upstream source", src.Close()),
		)
	}

	log.Info().
		Int64("processed", p.processed.Load()).
		Int64("failed", p.failed.Load()).
		Int64("deadLettered", p.deadLettered.Load()).
		Msg("stream bridge statistics")

	return err
}

type pipeline struct {
	sink  sink.Sink
	dlq   dlq.Writer
	cfg   config.Bridge
	retry retry.Policy

	runCtx context.Context
	cancel context.CancelFunc

	failOnce sync.Once
	failMu   sync.Mutex
	failErr  error

	processed    atomic.Int64
	failed       atomic.Int64
	deadLettered atomic.Int64
}

func (p *pipeline) work(ctx context.Context, workerID int, jobs <-chan source.Message) {
	log.Debug().Int("worker", workerID).Msg("worker started")
	defer log.Debug().Int("worker", workerID).Msg("worker stopped")

	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-jobs:
			if !ok {
				return
			}
			if ctx.Err() != nil {
				return
			}
			p.handle(ctx, message)
		}
	}
}

func (p *pipeline) handle(ctx context.Context, message source.Message) {
	attempts := 0
	messageCtx, cancel := context.WithTimeout(ctx, p.cfg.MessageTimeout.Std())
	defer cancel()

	policy := p.retry
	policy.OnRetry = func(attempt int, err error) {
		log.Warn().
			Err(err).
			Str("source", message.Source).
			Str("topic", message.Topic).
			Str("queue", message.Queue).
			Int32("partition", message.Partition).
			Int64("offset", message.Offset).
			Int("attempt", attempt).
			Msg("downstream write failed; retrying")
	}
	policy.Abort = sink.IsPermanent

	err := policy.Do(messageCtx, func(ctx context.Context) error {
		attempts++
		return p.sink.Write(ctx, message.Value)
	})

	if err == nil {
		p.processed.Add(1)
		if ackErr := message.Ack(nil); ackErr != nil {
			p.fail(fmt.Errorf("acknowledge processed message: %w", ackErr))
		}
		return
	}

	// Shutdown interrupted this message. Leave it unacknowledged so the broker
	// redelivers it after restart.
	if p.runCtx.Err() != nil {
		return
	}

	p.failed.Add(1)
	terminalErr := fmt.Errorf("process %s message: %w", message.Source, err)
	permanent := sink.IsPermanent(err)

	if p.cfg.FailurePolicy != config.FailurePolicyDLQ {
		if ackErr := message.Ack(terminalErr); ackErr != nil {
			p.fail(errors.Join(terminalErr, fmt.Errorf("report failed message: %w", ackErr)))
			return
		}
		p.fail(terminalErr)
		return
	}

	record := dlq.Record{
		Source:      message.Source,
		Topic:       message.Topic,
		Partition:   message.Partition,
		Offset:      message.Offset,
		Queue:       message.Queue,
		DeliveryTag: message.DeliveryTag,
		Attempts:    attempts,
		Error:       err.Error(),
		Timestamp:   time.Now().UTC(),
		Payload:     message.Value,
	}

	dlqCtx, dlqCancel := context.WithTimeout(ctx, p.cfg.MessageTimeout.Std())
	defer dlqCancel()

	if dlqErr := p.dlq.Write(dlqCtx, record); dlqErr != nil {
		// A DLQ write interrupted by shutdown is not a bridge failure; the
		// original message stays unacknowledged and will be replayed.
		if p.runCtx.Err() != nil {
			return
		}
		ackErr := message.Ack(terminalErr)
		p.fail(errors.Join(
			terminalErr,
			fmt.Errorf("write dead letter: %w", dlqErr),
			wrapClose("report failed message", ackErr),
		))
		return
	}

	p.deadLettered.Add(1)
	log.Warn().
		Str("source", message.Source).
		Str("topic", message.Topic).
		Str("queue", message.Queue).
		Int32("partition", message.Partition).
		Int64("offset", message.Offset).
		Int("attempts", attempts).
		Bool("permanent", permanent).
		Msg("message written to dead-letter queue")

	if ackErr := message.Ack(nil); ackErr != nil {
		p.fail(fmt.Errorf("acknowledge dead-lettered message: %w", ackErr))
	}
}

func (p *pipeline) fail(err error) {
	p.failOnce.Do(func() {
		p.failMu.Lock()
		p.failErr = err
		p.failMu.Unlock()
		p.cancel()
	})
}

func (p *pipeline) failure() error {
	p.failMu.Lock()
	defer p.failMu.Unlock()
	return p.failErr
}

func retryPolicy(cfg config.RetryPolicy) retry.Policy {
	return retry.Policy{
		Attempts:  cfg.Attempts,
		BaseDelay: cfg.BaseDelay.Std(),
		MaxDelay:  cfg.MaxDelay.Std(),
	}
}

func withoutCancellation(err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func wrapClose(operation string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", operation, err)
}

func closeWriter(writer dlq.Writer) error {
	if writer == nil {
		return nil
	}
	return writer.Close()
}
