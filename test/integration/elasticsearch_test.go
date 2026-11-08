//go:build integration

package integration

import (
	"fmt"
	"testing"
)

func TestKafkaToElasticsearch(t *testing.T) {
	brokers := kafkaBrokers(t)
	esURL := elasticURL(t)

	inputTopic := uniqueName("elastic-input")
	group := uniqueName("elastic-group")
	index := newElasticIndex("elastic")
	cleanupElasticIndex(t, esURL, index)

	var values []string
	for i := 0; i < 6; i++ {
		values = append(values, fmt.Sprintf("elastic-%02d", i))
	}
	produceKafka(t, brokers, inputTopic, values...)

	configDir := writeConfigDir(t, map[string]any{
		"kafka_consumer.json": kafkaConsumerConfig(brokers, inputTopic, group),
		"elastic.json":        elasticConfig(esURL, index),
		"bridge.json":         bridgeConfig(2, stopPolicy(), nil),
	})

	bridge := startBridge(t, configDir, "kafka", "elastic")
	waitForElasticCount(t, esURL, index, len(values))
	requireSameMessages(t, values, elasticMessages(t, esURL, index))

	if err := bridge.stop(t); err != nil {
		t.Fatalf("bridge stopped with error: %v", err)
	}
	waitForCommittedOffsets(t, brokers, group, int64(len(values)))
}
