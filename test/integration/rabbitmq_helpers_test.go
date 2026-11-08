//go:build integration

package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func rabbitURL(t *testing.T) string {
	t.Helper()
	return requireEnv(t, "RABBITMQ_URL")
}

func dialRabbit(t *testing.T, url string) (*amqp.Connection, *amqp.Channel) {
	t.Helper()

	conn, err := amqp.Dial(url)
	if err != nil {
		t.Fatalf("dial rabbitmq: %v", err)
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		t.Fatalf("open rabbitmq channel: %v", err)
	}
	t.Cleanup(func() {
		_ = channel.Close()
		_ = conn.Close()
	})
	return conn, channel
}

func declareRabbitQueue(t *testing.T, channel *amqp.Channel, queue string) {
	t.Helper()
	if _, err := channel.QueueDeclare(queue, true, false, false, false, nil); err != nil {
		t.Fatalf("declare queue %s: %v", queue, err)
	}
}

func publishRabbit(t *testing.T, url, queue string, values ...string) {
	t.Helper()

	_, channel := dialRabbit(t, url)
	declareRabbitQueue(t, channel, queue)

	if err := channel.Confirm(false); err != nil {
		t.Fatalf("enable publisher confirms: %v", err)
	}
	confirms := channel.NotifyPublish(make(chan amqp.Confirmation, len(values)))

	for _, value := range values {
		err := channel.PublishWithContext(context.Background(),
			"",    // exchange
			queue, // routing key
			false, // mandatory
			false, // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(value),
			},
		)
		if err != nil {
			t.Fatalf("publish to %s: %v", queue, err)
		}
	}

	for range values {
		select {
		case confirmation := <-confirms:
			if !confirmation.Ack {
				t.Fatalf("rabbitmq rejected a publish to %s", queue)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("timed out waiting for publish confirmation on %s", queue)
		}
	}
}

func rabbitQueueDepth(t *testing.T, url, queue string) (int, error) {
	t.Helper()

	conn, err := amqp.Dial(url)
	if err != nil {
		return 0, fmt.Errorf("dial rabbitmq: %w", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		return 0, fmt.Errorf("open rabbitmq channel: %w", err)
	}
	defer channel.Close()

	declared, err := channel.QueueDeclarePassive(queue, true, false, false, false, nil)
	if err != nil {
		return 0, fmt.Errorf("inspect queue %s: %w", queue, err)
	}
	return declared.Messages, nil
}

func waitForRabbitQueueEmpty(t *testing.T, url, queue string) {
	t.Helper()
	waitFor(t, 20*time.Second, 200*time.Millisecond, fmt.Sprintf("rabbitmq queue %s draining", queue), func() (bool, string) {
		depth, err := rabbitQueueDepth(t, url, queue)
		if err != nil {
			return false, err.Error()
		}
		return depth == 0, fmt.Sprintf("ready messages = %d", depth)
	})
}
