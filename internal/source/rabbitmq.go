package source

import (
	"context"
	"errors"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"

	"gostreambridge/internal/config"
	"gostreambridge/internal/retry"
)

const amqpDefaultPrefetch = 10

// RabbitMQSource consumes messages from a RabbitMQ queue. The connection and
// channel are created once and reused for the lifetime of the process.
type RabbitMQSource struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	queue      string
	deliveries <-chan amqp.Delivery

	// AMQP channels are not safe for concurrent use; all acknowledgements and
	// channel shutdown are serialized through this mutex.
	ackMu     sync.Mutex
	closeOnce sync.Once
	closeErr  error
}

// NewRabbitMQSource loads the RabbitMQ configuration, connects to the broker,
// and starts a consumer with manual acknowledgements.
func NewRabbitMQSource(ctx context.Context, configDir string) (*RabbitMQSource, error) {
	cfg, err := config.Load[config.RabbitMQ](config.File(configDir, "rabbitmq.json"))
	if err != nil {
		return nil, err
	}

	conn, err := dialRabbitMQ(ctx, cfg.URL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("open rabbitmq channel: %w", err),
			conn.Close(),
		)
	}

	cleanup := func() error {
		return errors.Join(ch.Close(), conn.Close())
	}

	durable := true
	if cfg.Durable != nil {
		durable = *cfg.Durable
	}

	q, err := ch.QueueDeclare(
		cfg.Queue,      // name
		durable,        // durable
		cfg.AutoDelete, // delete when unused
		false,          // exclusive
		false,          // no-wait
		nil,            // arguments
	)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("declare rabbitmq queue %q: %w", cfg.Queue, err),
			cleanup(),
		)
	}

	prefetch := cfg.Prefetch
	if prefetch == 0 {
		prefetch = amqpDefaultPrefetch
	}
	qosErr := ch.Qos(prefetch, 0, false)
	if qosErr != nil {
		return nil, errors.Join(
			fmt.Errorf("set rabbitmq prefetch: %w", qosErr),
			cleanup(),
		)
	}

	deliveries, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack: ack only after successful processing
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return nil, errors.Join(
			fmt.Errorf("register rabbitmq consumer: %w", err),
			cleanup(),
		)
	}

	return &RabbitMQSource{
		conn:       conn,
		channel:    ch,
		queue:      q.Name,
		deliveries: deliveries,
	}, nil
}

// Consume processes deliveries until the context is cancelled. Messages are
// acknowledged only after the pipeline reports a terminal success; failures
// remain unacknowledged and are requeued when the channel closes.
func (s *RabbitMQSource) Consume(ctx context.Context, deliver Deliver) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case delivery, ok := <-s.deliveries:
			if !ok {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return errors.New("rabbitmq delivery channel closed")
			}

			delivered := Message{
				Value:       delivery.Body,
				Source:      "rabbitmq",
				Queue:       s.queue,
				DeliveryTag: delivery.DeliveryTag,
				Ack: func(processingErr error) error {
					if processingErr == nil {
						s.ackMu.Lock()
						defer s.ackMu.Unlock()
						return delivery.Ack(false)
					}
					// A processing error leaves the message unacknowledged for replay.
					return nil
				},
			}

			if err := deliver(delivered); err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				return fmt.Errorf("deliver rabbitmq message %d: %w", delivery.DeliveryTag, err)
			}
		}
	}
}

// Close releases the RabbitMQ channel and connection. Unacknowledged messages
// are requeued by the broker.
func (s *RabbitMQSource) Close() error {
	s.closeOnce.Do(func() {
		s.ackMu.Lock()
		defer s.ackMu.Unlock()
		s.closeErr = errors.Join(s.channel.Close(), s.conn.Close())
	})
	return s.closeErr
}

// dialRabbitMQ connects to RabbitMQ with exponential backoff.
func dialRabbitMQ(ctx context.Context, url string) (*amqp.Connection, error) {
	policy := retry.Default()
	policy.OnRetry = func(attempt int, err error) {
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Msg("failed to connect to rabbitmq")
	}

	var conn *amqp.Connection
	err := policy.Do(ctx, func(context.Context) error {
		var dialErr error
		conn, dialErr = amqp.Dial(url)
		return dialErr
	})
	if err != nil {
		return nil, fmt.Errorf("connect to rabbitmq: %w", err)
	}

	return conn, nil
}
