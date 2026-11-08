package dlq

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileWriter appends newline-delimited JSON records to a file. It is a durable
// fallback for deployments without a Kafka DLQ topic.
type FileWriter struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileWriter opens or creates the DLQ file, creating parent directories.
func NewFileWriter(path string) (*FileWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("create DLQ directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open DLQ file %q: %w", path, err)
	}
	return &FileWriter{file: file}, nil
}

// Write appends one JSON line and syncs it to disk.
func (w *FileWriter) Write(ctx context.Context, record Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	payload, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal DLQ record: %w", err)
	}
	payload = append(payload, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Write(payload); err != nil {
		return fmt.Errorf("write DLQ record: %w", err)
	}
	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("sync DLQ record: %w", err)
	}
	return nil
}

// Close releases the file handle.
func (w *FileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Close()
}
