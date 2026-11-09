// Command register demonstrates application-owned connector registration.
package main

import (
	"gostreambridge/examples/cdk/connector"
	"gostreambridge/pkg/bridge/registry"
)

func main() {
	registry.RegisterSink("example-log", connector.Factory)
}
