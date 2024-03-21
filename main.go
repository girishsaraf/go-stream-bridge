package main

import (
	"flag"
	"os"

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
	downstreamApp := flag.String("downstreamApp", "", "Provide the downstream app name (mysql / elastic / kafka)")

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

	stream.StartStreamBridge(argMap)
}
