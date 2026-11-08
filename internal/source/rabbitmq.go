package source

import (
	"context"
	"errors"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/rs/zerolog/log"

	"gostreambridge/internal/config"
	"gostreambridge/internal/retry"
)

const (
	amqpPrefetch     = 10
	amqpRequeueDelay = 2 * time.Second
)

// RabbitMQSource consumes messages from a RabbitMQ queue. The connection and
// channel are created once and reused for the lifetime of the process.
type RabbitMQSource struct {
	conn       *amqp.Connection
	channel    *amqp.Channel
	queue      string
	deliveries <-chan amqp.Delivery
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
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	cleanup := func() {
		_ = ch.Close()
		_ = conn.Close()
	}

	q, err := ch.QueueDeclare(
		cfg.Queue, // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("declare rabbitmq queue %q: %w", cfg.Queue, err)
	}

	if err := ch.Qos(amqpPrefetch, 0, false); err != nil {
		cleanup()
		return nil, fmt.Errorf("set rabbitmq prefetch: %w", err)
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
		cleanup()
		return nil, fmt.Errorf("register rabbitmq consumer: %w", err)
	}

	return &RabbitMQSource{
		conn:       conn,
		channel:    ch,
		queue:      q.Name,
		deliveries: deliveries,
	}, nil
}

// Consume processes deliveries until the context is cancelled. Messages are
// acknowledged only after the handler succeeds; failures are requeued.
func (s *RabbitMQSource) Consume(ctx context.Context, handle Handler) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-s.deliveries:
			if !ok {
				return errors.New("rabbitmq delivery channel closed")
			}

			if err := handle(ctx, msg.Body); err != nil {
				// Closing the connection requeues unacked deliveries.
				if ctx.Err() != nil {
					return nil
				}
				log.Error().Err(err).Str("queue", s.queue).Msg("failed to process rabbitmq message; requeueing")
				if nackErr := msg.Nack(false, true); nackErr != nil {
					return fmt.Errorf("nack rabbitmq message: %w", nackErr)
				}
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(amqpRequeueDelay):
				}
				continue
			}

			if err := msg.Ack(false); err != nil {
				return fmt.Errorf("ack rabbitmq message: %w", err)
			}
		}
	}
}

// Close releases the RabbitMQ channel and connection.
func (s *RabbitMQSource) Close() error {
	return errors.Join(s.channel.Close(), s.conn.Close())
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
