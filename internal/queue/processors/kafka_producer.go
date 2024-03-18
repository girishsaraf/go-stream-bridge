package processors

import (
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"log"

	"gostreambridge/pkg/util"
)

// WriteToKafka writes messages to Kafka topic
func WriteToKafka(message string) error {

	// Reading configuration
	kafkaConfig := util.ConvertConfigFileToMap("kafka_producer.json")

	// Kafka producer configuration
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaConfig["broker"],
	})
	if err != nil {
		return err
	}
	defer producer.Close()

	topic := kafkaConfig["topic"]

	// Produce message to Kafka topic
	deliveryChan := make(chan kafka.Event)
	err = producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          []byte(message),
	}, deliveryChan)
	if err != nil {
		return err
	}

	// Wait for message delivery report
	e := <-deliveryChan
	m := e.(*kafka.Message)
	if m.TopicPartition.Error != nil {
		return m.TopicPartition.Error
	}

	log.Printf("Message written to Kafka topic: %s\n", topic)
	return nil
}
