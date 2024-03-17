package gostreambridge

import (
	"fmt"
	"gostreambridge/internal/stream"
)

func main(){
	fmt.Printf("Initialized flow")
	stream.StartStreamBridge("kafka", "mysql")
}