package processors

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"log"
	"time"

	"gostreambridge/pkg/util"
)

// WriteToKafka writes messages to Kafka topic
func WriteToKafka(message string) error {

	// Reading configuration
	kafkaConfig, _ := util.ConvertConfigFileToMap("kafka_producer.json")

	// Kafka producer configuration
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaConfig["broker"],
	})
	if err != nil {
		return err
	}
	defer producer.Close()

	topic := kafkaConfig["topic"]
	maxRetries := 5

	// Produce message to Kafka topic with retries
	for i := 0; i < maxRetries; i++ {
		deliveryChan := make(chan kafka.Event)
		err := producer.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          []byte(message),
		}, deliveryChan)
		if err != nil {
			return err
		}

		select {
		case e := <-deliveryChan:
			m := e.(*kafka.Message)
			if m.TopicPartition.Error != nil {
				log.Printf("Failed to deliver message to Kafka topic %s: %v, retrying...\n", topic, m.TopicPartition.Error)
				continue
			}
			log.Printf("Message written to Kafka topic: %s\n", topic)
			return nil
		case <-time.After(5 * time.Second):
			log.Println("Message delivery timed out, retrying...")
			continue
		}
	}

	return kafka.NewError(kafka.ErrAllBrokersDown, "All retries exhausted", true)
}
