package dlq

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gostreambridge/internal/config"
)

func TestNewDisabled(t *testing.T) {
	writer, err := New(config.DLQ{Type: config.DLQTypeNone})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if writer != nil {
		t.Fatal("disabled DLQ should return a nil writer")
	}
}

func TestNewRejectsUnknownType(t *testing.T) {
	if _, err := New(config.DLQ{Type: "unknown"}); err == nil {
		t.Fatal("expected an error")
	}
}

func TestFileWriterAppendsRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "dead-letters.jsonl")
	writer, err := NewFileWriter(path)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	records := []Record{
		{Source: "kafka", Topic: "input", Partition: 2, Offset: 42, Attempts: 3, Error: "boom", Timestamp: now, Payload: []byte("one")},
		{Source: "rabbitmq", Queue: "jobs", DeliveryTag: 7, Attempts: 1, Error: "bad", Timestamp: now, Payload: []byte("two")},
	}
	for _, record := range records {
		if err := writer.Write(context.Background(), record); err != nil {
			t.Fatalf("Write: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open DLQ file: %v", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close DLQ file: %v", closeErr)
		}
	}()

	var got []Record
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var record Record
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatalf("parse DLQ line: %v", err)
		}
		got = append(got, record)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan DLQ file: %v", err)
	}

	if len(got) != len(records) {
		t.Fatalf("expected %d records, got %d", len(records), len(got))
	}
	if string(got[0].Payload) != "one" || got[0].Offset != 42 {
		t.Errorf("unexpected first record: %+v", got[0])
	}
	if string(got[1].Payload) != "two" || got[1].DeliveryTag != 7 {
		t.Errorf("unexpected second record: %+v", got[1])
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat DLQ file: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("expected 0600 permissions, got %o", info.Mode().Perm())
	}
}

func TestFileWriterHonorsCancelledContext(t *testing.T) {
	writer, err := NewFileWriter(filepath.Join(t.TempDir(), "dlq.jsonl"))
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			t.Errorf("Close: %v", closeErr)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err = writer.Write(ctx, Record{Payload: []byte("x")})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
