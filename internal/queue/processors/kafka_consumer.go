package processors

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gostreambridge/pkg/util"
)

func ConsumeKafkaMessages() <-chan *kafka.Message {
	messages := make(chan *kafka.Message)

	// Reading configuration
	kafkaConfig, _ := util.ConvertConfigFileToMap("kafka_consumer.json")

	// Kafka consumer configuration
	config := kafka.ConfigMap{
		"bootstrap.servers":  kafkaConfig["brokers"],
		"group.id":           kafkaConfig["groupId"],
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": "false",
	}

	// Create Kafka consumer
	consumer, err := kafka.NewConsumer(&config)
	if err != nil {
		log.Fatalf("Failed to create consumer: %s\n", err)
	}
	defer consumer.Close()

	// Subscribe to topic(s)
	topic := kafkaConfig["topic"]
	err = consumer.SubscribeTopics([]string{topic}, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to topic: %s\n", err)
	}

	// Channel to handle OS signals
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	maxRetries := 5

	// Start consuming messages
	go func() {
	ConsumerLoop:
		for {
			select {
			case sig := <-sigchan:
				log.Printf("Caught signal %v: terminating\n", sig)
				break ConsumerLoop
			default:
				for i := 0; i < maxRetries; i++ {
					msg, err := consumer.ReadMessage(-1)
					if err != nil {
						log.Printf("Error reading message from Kafka: %v, retrying...\n", err)
						continue
					}
					messages <- msg
					break
				}
				time.Sleep(5 * time.Second) // Wait before retrying
			}
		}
	}()

	return messages
}
