package sink

import (
	"errors"
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func TestClassifyKafkaError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		permanent bool
	}{
		{
			name:      "message too large",
			err:       kafka.NewError(kafka.ErrMsgSizeTooLarge, "too large", false),
			permanent: true,
		},
		{
			name:      "topic authorization",
			err:       kafka.NewError(kafka.ErrTopicAuthorizationFailed, "denied", false),
			permanent: true,
		},
		{
			name: "transient broker error",
			err:  kafka.NewError(kafka.ErrTransport, "connection refused", false),
		},
		{
			name: "generic error",
			err:  errors.New("timeout"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := classifyKafkaError(test.err)
			if !errors.Is(err, test.err) {
				t.Fatalf("classification lost the original error: %v", err)
			}
			if got := IsPermanent(err); got != test.permanent {
				t.Fatalf("permanent=%v, want %v", got, test.permanent)
			}
		})
	}
}
