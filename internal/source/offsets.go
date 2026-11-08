package source

import (
	"sync"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

// offsetTracker computes contiguous Kafka commit positions while messages are
// processed concurrently and out of order. It also tolerates offsets skipped
// by log compaction: an offset is only considered skipped once a later offset
// has been delivered.
type offsetTracker struct {
	topic      string
	mu         sync.Mutex
	partitions map[int32]*partitionOffsets
}

type partitionOffsets struct {
	initialized      bool
	next             int64
	highestDelivered int64
	delivered        map[int64]struct{}
	completed        map[int64]struct{}
	dirty            bool
}

func newOffsetTracker(topic string) *offsetTracker {
	return &offsetTracker{
		topic:      topic,
		partitions: make(map[int32]*partitionOffsets),
	}
}

func (t *offsetTracker) partition(partition int32) *partitionOffsets {
	state, ok := t.partitions[partition]
	if !ok {
		state = &partitionOffsets{
			delivered: make(map[int64]struct{}),
			completed: make(map[int64]struct{}),
		}
		t.partitions[partition] = state
	}
	return state
}

// Delivered records that offset was handed to the pipeline. It must be called
// in broker order for the partition.
func (t *offsetTracker) Delivered(partition int32, offset int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state := t.partition(partition)
	if !state.initialized {
		state.initialized = true
		state.next = offset
		state.highestDelivered = offset
	}
	if offset > state.highestDelivered {
		state.highestDelivered = offset
	}
	state.delivered[offset] = struct{}{}
	t.advance(state)
}

// Mark records a successfully handled offset. Offsets for partitions that are
// no longer tracked (for example, after a rebalance revoked them) are ignored
// so a late acknowledgement cannot recreate stale commit state.
func (t *offsetTracker) Mark(partition int32, offset int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.partitions[partition]
	if !ok {
		return
	}
	if offset < state.next {
		return
	}

	delete(state.delivered, offset)
	state.completed[offset] = struct{}{}
	t.advance(state)
}

// advance moves next past every completed offset and every gap that can no
// longer be delivered. The caller must hold the tracker lock.
func (t *offsetTracker) advance(state *partitionOffsets) {
	for {
		if _, ok := state.completed[state.next]; ok {
			delete(state.completed, state.next)
			state.next++
			state.dirty = true
			continue
		}

		// Offsets below the highest delivered offset that were never delivered
		// were skipped (for example by compaction) and are safe to pass.
		if state.next < state.highestDelivered {
			if _, ok := state.delivered[state.next]; !ok {
				state.next++
				state.dirty = true
				continue
			}
		}
		return
	}
}

// Pending returns the next commit position for each partition that advanced
// since its last successful commit.
func (t *offsetTracker) Pending() []kafka.TopicPartition {
	t.mu.Lock()
	defer t.mu.Unlock()

	partitions := make([]kafka.TopicPartition, 0, len(t.partitions))
	for partition, state := range t.partitions {
		if !state.dirty {
			continue
		}
		topic := t.topic
		partitions = append(partitions, kafka.TopicPartition{
			Topic:     &topic,
			Partition: partition,
			Offset:    kafka.Offset(state.next),
		})
	}
	return partitions
}

// Committed marks a flushed position as persisted. If the tracker advanced
// again while the commit was in flight, the partition stays dirty.
func (t *offsetTracker) Committed(partition int32, offset int64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, ok := t.partitions[partition]
	if !ok {
		return
	}
	if state.next == offset {
		state.dirty = false
	}
}

// Forget drops tracking state for partitions that are no longer assigned.
func (t *offsetTracker) Forget(partitions []kafka.TopicPartition) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, partition := range partitions {
		delete(t.partitions, partition.Partition)
	}
}
