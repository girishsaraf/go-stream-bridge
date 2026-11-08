package sink

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/elastic/go-elasticsearch/v8"
)

func newTestElasticSink(t *testing.T, status int) *ElasticSink {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Elastic-Product", "Elasticsearch")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"error":"test failure"}`))
	}))
	t.Cleanup(server.Close)

	client, err := elasticsearch.NewClient(elasticsearch.Config{Addresses: []string{server.URL}})
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return &ElasticSink{client: client, index: "test-index"}
}

func TestElasticWriteClassifiesResponses(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		permanent bool
	}{
		{name: "created", status: http.StatusCreated},
		{name: "bad request", status: http.StatusBadRequest, permanent: true},
		{name: "unauthorized", status: http.StatusUnauthorized, permanent: true},
		{name: "forbidden", status: http.StatusForbidden, permanent: true},
		{name: "not found", status: http.StatusNotFound, permanent: true},
		{name: "request timeout", status: http.StatusRequestTimeout},
		{name: "too many requests", status: http.StatusTooManyRequests},
		{name: "server error", status: http.StatusInternalServerError},
		{name: "unavailable", status: http.StatusServiceUnavailable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			s := newTestElasticSink(t, test.status)
			err := s.Write(context.Background(), []byte("hello"))
			if test.status == http.StatusCreated {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := IsPermanent(err); got != test.permanent {
				t.Fatalf("permanent=%v, want %v (err=%v)", got, test.permanent, err)
			}
		})
	}
}

func TestElasticWriteHonorsCancelledContext(t *testing.T) {
	s := newTestElasticSink(t, http.StatusCreated)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := s.Write(ctx, []byte("hello"))
	if err == nil {
		t.Fatal("expected an error")
	}
	if IsPermanent(err) {
		t.Fatal("cancellation must remain retryable")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
