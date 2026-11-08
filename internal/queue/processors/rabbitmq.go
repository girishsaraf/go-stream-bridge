package processors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/streadway/amqp"

	"gostreambridge/pkg/util"
)

const (
	amqpConnectAttempts = 5
	amqpRetryDelay      = 2 * time.Second
	amqpPrefetch        = 10
)

// ConsumeAMQPMessages consumes messages from RabbitMQ until the context is
// cancelled or a fatal error occurs. Messages are acknowledged only after the
// handler succeeds; failures are requeued.
func ConsumeAMQPMessages(ctx context.Context, handler func([]byte) error) error {
	// Reading configuration
	amqpConfig, err := util.ConvertConfigFileToMap("rabbitmq.json")
	if err != nil {
		return fmt.Errorf("load rabbitmq config: %w", err)
	}

	url := amqpConfig["url"]
	if url == "" {
		return errors.New("rabbitmq config: url must not be empty")
	}

	conn, err := dialAMQP(ctx, url)
	if err != nil {
		return err
	}
	// Deferred closes run when the consume loop exits.
	defer conn.Close()

	// Create a channel
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer ch.Close()

	// Declare a queue
	q, err := ch.QueueDeclare(
		amqpConfig["queue"], // name
		false,               // durable
		false,               // delete when unused
		false,               // exclusive
		false,               // no-wait
		nil,                 // arguments
	)
	if err != nil {
		return fmt.Errorf("declare rabbitmq queue %q: %w", amqpConfig["queue"], err)
	}

	// Bound the number of unacknowledged deliveries this consumer may hold.
	if err := ch.Qos(amqpPrefetch, 0, false); err != nil {
		return fmt.Errorf("set rabbitmq prefetch: %w", err)
	}

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		false,  // auto-ack: ack only after successful processing
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	if err != nil {
		return fmt.Errorf("register rabbitmq consumer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return errors.New("rabbitmq delivery channel closed")
			}

			if err := handler(msg.Body); err != nil {
				// Closing the connection requeues unacked deliveries.
				if ctx.Err() != nil {
					return nil
				}
				log.Error().Err(err).Str("queue", q.Name).Msg("failed to process rabbitmq message; requeueing")
				if nackErr := msg.Nack(false, true); nackErr != nil {
					return fmt.Errorf("nack rabbitmq message: %w", nackErr)
				}
				// Avoid a tight requeue loop while the downstream is unhealthy.
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(amqpRetryDelay):
				}
				continue
			}

			if err := msg.Ack(false); err != nil {
				return fmt.Errorf("ack rabbitmq message: %w", err)
			}
		}
	}
}

// dialAMQP connects to RabbitMQ with bounded retries, aborting early if the
// context is cancelled.
func dialAMQP(ctx context.Context, url string) (*amqp.Connection, error) {
	var lastErr error
	for attempt := 1; attempt <= amqpConnectAttempts; attempt++ {
		conn, err := amqp.Dial(url)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		log.Warn().
			Err(err).
			Int("attempt", attempt).
			Int("maxAttempts", amqpConnectAttempts).
			Msg("failed to connect to rabbitmq")

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(amqpRetryDelay):
		}
	}

	return nil, fmt.Errorf("connect to rabbitmq after %d attempts: %w", amqpConnectAttempts, lastErr)
}
