package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"gostreambridge/internal/stream"
)

const defaultConfigDir = "internal/config/files"

// Set by the build via -ldflags.
var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

func main() {
	configureLogging()

	upstreamApp := flag.String("upstreamApp", "", "upstream source: kafka or rabbitmq")
	downstreamApp := flag.String("downstreamApp", "", "downstream sink: kafka, mysql, sqlserver, or elastic")
	configDir := flag.String("configDir", envOrDefault("STREAM_BRIDGE_CONFIG_DIR", defaultConfigDir), "directory containing JSON configuration files")
	showVersion := flag.Bool("version", false, "print version information and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("stream-bridge version=%s commit=%s buildDate=%s go=%s\n", version, commit, buildDate, runtime.Version())
		return
	}

	if *upstreamApp == "" || *downstreamApp == "" {
		log.Fatal().Msg("usage: stream-bridge -upstreamApp=kafka -downstreamApp=mysql [-configDir=path]")
	}

	// A single signal handler owns shutdown. Cancelling the context stops the
	// source, drains in-flight work, and closes every long-lived client. Once
	// shutdown starts, restore the default signal behavior so a second signal
	// can force-quit a drain that is taking too long.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		stop()
	}()

	log.Info().
		Str("version", version).
		Str("commit", commit).
		Str("upstream", *upstreamApp).
		Str("downstream", *downstreamApp).
		Str("configDir", *configDir).
		Msg("starting stream bridge")

	opts := stream.Options{
		UpstreamApp:   *upstreamApp,
		DownstreamApp: *downstreamApp,
		ConfigDir:     *configDir,
	}

	if err := stream.StartStreamBridge(ctx, opts); err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal().Err(err).Msg("stream bridge failed")
	}

	log.Info().Msg("stream bridge stopped")
}

func configureLogging() {
	level := zerolog.InfoLevel
	if raw := os.Getenv("LOG_LEVEL"); raw != "" {
		parsed, err := zerolog.ParseLevel(raw)
		if err != nil {
			log.Warn().Str("value", raw).Msg("invalid LOG_LEVEL; using info")
		} else {
			level = parsed
		}
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = time.RFC3339Nano

	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "json") {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Caller().Logger()
		return
	}

	log.Logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Caller().Logger()
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
