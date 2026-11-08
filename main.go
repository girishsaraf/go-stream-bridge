package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gostreambridge/internal/stream"
)

func main() {
	// Set up zerolog with console output and human-readable format
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	log.Info().Msg("Initializing stream bridge")

	// Define flags for two arguments
	upstreamApp := flag.String("upstreamApp", "", "Provide the upstream app name (kafka / rabbitmq)")
	downstreamApp := flag.String("downstreamApp", "", "Provide the downstream app name (mysql / sqlserver / elastic / kafka)")

	// Parse command-line arguments
	flag.Parse()

	// Check if both arguments are provided
	if *upstreamApp == "" || *downstreamApp == "" {
		log.Fatal().Msg("Usage: go run main.go -upstreamApp=valueX -downstreamApp=valueY")
	}

	// Convert arguments to map
	argMap := map[string]string{
		"upstreamApp":   *upstreamApp,
		"downstreamApp": *downstreamApp,
	}

	// A single signal handler owns shutdown. Cancelling the context stops the
	// consumer loop and closes its resources via deferred calls.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := stream.StartStreamBridge(ctx, argMap); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal().Err(err).Msg("stream bridge failed")
	}

	log.Info().Msg("Stream bridge stopped")
}
