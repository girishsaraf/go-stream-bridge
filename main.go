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

const defaultConfigDir = "internal/config/files"

func main() {
	// Set up zerolog with console output and human-readable format
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()

	log.Info().Msg("Initializing stream bridge")

	upstreamApp := flag.String("upstreamApp", "", "Provide the upstream app name (kafka / rabbitmq)")
	downstreamApp := flag.String("downstreamApp", "", "Provide the downstream app name (mysql / sqlserver / elastic / kafka)")
	configDir := flag.String("configDir", envOrDefault("STREAM_BRIDGE_CONFIG_DIR", defaultConfigDir), "Directory containing JSON configuration files")
	flag.Parse()

	if *upstreamApp == "" || *downstreamApp == "" {
		log.Fatal().Msg("Usage: go run main.go -upstreamApp=valueX -downstreamApp=valueY [-configDir=path]")
	}

	// A single signal handler owns shutdown. Cancelling the context stops the
	// source and closes both the source and sink resources.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	opts := stream.Options{
		UpstreamApp:   *upstreamApp,
		DownstreamApp: *downstreamApp,
		ConfigDir:     *configDir,
	}

	if err := stream.StartStreamBridge(ctx, opts); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal().Err(err).Msg("stream bridge failed")
	}

	log.Info().Msg("Stream bridge stopped")
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
