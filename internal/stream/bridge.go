// Package stream wires an upstream source to a downstream sink.
package stream

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"gostreambridge/internal/sink"
	"gostreambridge/internal/source"
)

// Options describes the bridge selected on the command line.
type Options struct {
	UpstreamApp   string
	DownstreamApp string
	ConfigDir     string
}

// StartStreamBridge constructs the configured source and sink and runs the
// bridge until the context is cancelled or either component fails.
func StartStreamBridge(ctx context.Context, opts Options) error {
	log.Info().
		Str("upstream", opts.UpstreamApp).
		Str("downstream", opts.DownstreamApp).
		Str("configDir", opts.ConfigDir).
		Msg("starting stream bridge")

	src, err := source.New(ctx, opts.UpstreamApp, opts.ConfigDir)
	if err != nil {
		return fmt.Errorf("create upstream source: %w", err)
	}

	dst, err := sink.New(ctx, opts.DownstreamApp, opts.ConfigDir)
	if err != nil {
		if closeErr := src.Close(); closeErr != nil {
			return errors.Join(
				fmt.Errorf("create downstream sink: %w", err),
				fmt.Errorf("close upstream source: %w", closeErr),
			)
		}
		return fmt.Errorf("create downstream sink: %w", err)
	}

	return Run(ctx, src, dst)
}

// Run consumes from src and writes to dst. Both components are closed before
// Run returns; their close errors are joined with the consumption error.
func Run(ctx context.Context, src source.Source, dst sink.Sink) (err error) {
	defer func() {
		err = errors.Join(err, src.Close(), dst.Close())
	}()

	return src.Consume(ctx, dst.Write)
}
