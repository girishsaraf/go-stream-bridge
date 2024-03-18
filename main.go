package main

import (
	"log"
	"flag"
	"gostreambridge/internal/stream"
)

func main() {
	log.Printf("Initializing stream bridge")

	// Define flags for two arguments
	upstreamApp := flag.String("upstreamApp", "", "Provide the upstream app name (kafka / rabbitmq)")
	downstreamApp := flag.String("downstreamApp", "", "Provide the downstream app name (mysql / elastic / kafka)")

	// Parse command-line arguments
	flag.Parse()

	// Check if both arguments are provided
	if *upstreamApp == "" || *downstreamApp == "" {
		log.Printf("Usage: go run main.go -upstreamApp=valueX -downstreamApp=valueY")
		return
	}

	// Convert arguments to map
	argMap := make(map[string]string)
	argMap["upstreamApp"] = *upstreamApp
	argMap["downstreamApp"] = *downstreamApp


	stream.StartStreamBridge(argMap)
}