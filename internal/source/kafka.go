package source

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/rs/zerolog/log"

	"gostreambridge/internal/config"
)

const (
	kafkaPollTimeout    = 500 * time.Millisecond
	kafkaCommitInterval = 500 * time.Millisecond
)

// KafkaSource consumes messages from a Kafka topic. The consumer is created
// once and reused for the lifetime of the process. Offsets are committed
// periodically at the highest contiguous processed position per partition.
type KafkaSource struct {
	consumer *kafka.Consumer
	tracker  *offsetTracker

	commitMu sync.Mutex

	done     chan struct{}
	doneOnce sync.Once
	loopDone chan struct{}
	closeErr error
}

// NewKafkaSource loads the consumer configuration and creates a subscribed
// Kafka consumer.
func NewKafkaSource(configDir string) (*KafkaSource, error) {
	cfg, err := config.Load[config.Kafka](config.File(configDir, "kafka_consumer.json"))
	if err != nil {
		return nil, err
	}
	if err := cfg.ValidateConsumer(); err != nil {
		return nil, fmt.Errorf("kafka consumer config: %w", err)
	}

	s := &KafkaSource{
		tracker:  newOffsetTracker(cfg.Topic),
		done:     make(chan struct{}),
		loopDone: make(chan struct{}),
	}

	consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        cfg.Brokers,
		"group.id":                 cfg.GroupID,
		"auto.offset.reset":        "earliest",
		"enable.auto.commit":       "false",
		"enable.auto.offset.store": "false",
	})
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}
	s.consumer = consumer

	if err := consumer.SubscribeTopics([]string{cfg.Topic}, s.rebalance); err != nil {
		_ = consumer.Close()
		return nil, fmt.Errorf("subscribe to kafka topic %q: %w", cfg.Topic, err)
	}

	go s.commitLoop()
	return s, nil
}

// Consume reads messages until the context is cancelled. Each message is
// handed to the pipeline with an Ack callback; offset commits happen only for
// contiguous successfully handled positions.
func (s *KafkaSource) Consume(ctx context.Context, deliver Deliver) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		msg, err := s.consumer.ReadMessage(kafkaPollTimeout)
		if err != nil {
			var kafkaErr kafka.Error
			if errors.As(err, &kafkaErr) {
				if kafkaErr.Code() == kafka.ErrTimedOut {
					continue
				}
				if kafkaErr.IsFatal() {
					return fmt.Errorf("fatal kafka consumer error: %w", err)
				}
			}
			if ctx.Err() != nil {
				return nil
			}
			log.Error().Err(err).Msg("error reading message from kafka")
			continue
		}

		partition := msg.TopicPartition.Partition
		offset := int64(msg.TopicPartition.Offset)
		topic := ""
		if msg.TopicPartition.Topic != nil {
			topic = *msg.TopicPartition.Topic
		}

		s.tracker.Delivered(partition, offset)

		delivered := Message{
			Value:     msg.Value,
			Source:    "kafka",
			Topic:     topic,
			Partition: partition,
			Offset:    offset,
			Ack: func(processingErr error) error {
				if processingErr != nil {
					// Leave the offset uncommitted so it is replayed.
					return nil
				}
				s.tracker.Mark(partition, offset)
				return nil
			},
		}

		if err := deliver(delivered); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("deliver kafka message %s: %w", msg.TopicPartition, err)
		}
	}
}

// Close stops offset commits, flushes the final contiguous position, and
// releases the consumer.
func (s *KafkaSource) Close() error {
	s.doneOnce.Do(func() {
		close(s.done)
		<-s.loopDone
		s.commitPending(nil)
		s.closeErr = s.consumer.Close()
	})
	return s.closeErr
}

func (s *KafkaSource) commitLoop() {
	defer close(s.loopDone)

	ticker := time.NewTicker(kafkaCommitInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.commitPending(nil)
		}
	}
}

// commitPending flushes dirty partitions, optionally restricted to the given
// partition set (used when partitions are revoked).
func (s *KafkaSource) commitPending(only map[int32]bool) {
	// The snapshot and commit must be serialized together. Otherwise a ticker
	// commit could race a rebalance commit and flush an older position after a
	// newer one.
	s.commitMu.Lock()
	defer s.commitMu.Unlock()

	partitions := s.tracker.Pending()
	if len(partitions) == 0 {
		return
	}
	if only != nil {
		filtered := partitions[:0]
		for _, partition := range partitions {
			if only[partition.Partition] {
				filtered = append(filtered, partition)
			}
		}
		partitions = filtered
	}
	if len(partitions) == 0 {
		return
	}

	if _, err := s.consumer.CommitOffsets(partitions); err != nil {
		log.Warn().Err(err).Msg("failed to commit kafka offsets; will retry")
		return
	}
	for _, partition := range partitions {
		s.tracker.Committed(partition.Partition, int64(partition.Offset))
	}
}

// rebalance commits completed positions before partitions are revoked. In-flight
// messages on revoked partitions may be replayed by the next owner, which is
// consistent with at-least-once delivery.
func (s *KafkaSource) rebalance(_ *kafka.Consumer, event kafka.Event) error {
	switch e := event.(type) {
	case kafka.RevokedPartitions:
		only := make(map[int32]bool, len(e.Partitions))
		for _, partition := range e.Partitions {
			only[partition.Partition] = true
		}
		s.commitPending(only)
		s.tracker.Forget(e.Partitions)
		log.Info().Int("partitions", len(e.Partitions)).Msg("kafka partitions revoked")
	case kafka.AssignedPartitions:
		log.Info().Int("partitions", len(e.Partitions)).Msg("kafka partitions assigned")
	}
	return nil
}
