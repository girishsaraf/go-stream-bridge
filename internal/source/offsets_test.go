package source

import (
	"testing"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func pendingOffsets(t *testing.T, tracker *offsetTracker) map[int32]int64 {
	t.Helper()

	result := make(map[int32]int64)
	for _, partition := range tracker.Pending() {
		result[partition.Partition] = int64(partition.Offset)
	}
	return result
}

func TestOffsetTrackerCommitsInOrder(t *testing.T) {
	tracker := newOffsetTracker("topic")
	tracker.Delivered(0, 10)
	if got := pendingOffsets(t, tracker); len(got) != 0 {
		t.Fatalf("expected no commit before processing, got %v", got)
	}

	tracker.Mark(0, 10)
	if got := pendingOffsets(t, tracker)[0]; got != 11 {
		t.Fatalf("expected commit position 11, got %d", got)
	}

	tracker.Committed(0, 11)
	if got := pendingOffsets(t, tracker); len(got) != 0 {
		t.Fatalf("expected committed position to be clean, got %v", got)
	}
}

func TestOffsetTrackerWaitsForEarlierOffsets(t *testing.T) {
	tracker := newOffsetTracker("topic")
	for offset := int64(10); offset < 13; offset++ {
		tracker.Delivered(0, offset)
	}

	tracker.Mark(0, 12)
	if got := pendingOffsets(t, tracker); len(got) != 0 {
		t.Fatalf("must not commit past an incomplete offset, got %v", got)
	}

	tracker.Mark(0, 10)
	if got := pendingOffsets(t, tracker)[0]; got != 11 {
		t.Fatalf("expected commit position 11, got %d", got)
	}

	tracker.Mark(0, 11)
	if got := pendingOffsets(t, tracker)[0]; got != 13 {
		t.Fatalf("expected commit position 13 after draining, got %d", got)
	}
}

func TestOffsetTrackerSkipsCompactedGaps(t *testing.T) {
	tracker := newOffsetTracker("topic")
	tracker.Delivered(0, 10)
	tracker.Delivered(0, 15)

	tracker.Mark(0, 10)
	if got := pendingOffsets(t, tracker)[0]; got != 15 {
		t.Fatalf("expected commit position to skip the gap to 15, got %d", got)
	}

	tracker.Mark(0, 15)
	if got := pendingOffsets(t, tracker)[0]; got != 16 {
		t.Fatalf("expected commit position 16, got %d", got)
	}
}

func TestOffsetTrackerIgnoresDuplicateAndOldMarks(t *testing.T) {
	tracker := newOffsetTracker("topic")
	tracker.Delivered(0, 10)
	tracker.Mark(0, 10)
	tracker.Mark(0, 10)
	tracker.Mark(0, 9)

	if got := pendingOffsets(t, tracker)[0]; got != 11 {
		t.Fatalf("expected commit position 11, got %d", got)
	}
}

func TestOffsetTrackerKeepsDirtyWhenCommitRaces(t *testing.T) {
	tracker := newOffsetTracker("topic")
	tracker.Delivered(0, 10)
	tracker.Mark(0, 10)

	snapshot := pendingOffsets(t, tracker)[0]
	tracker.Mark(0, 11) // advances while the commit for 11 is in flight

	tracker.Committed(0, snapshot)
	if got := pendingOffsets(t, tracker)[0]; got != 12 {
		t.Fatalf("expected a follow-up commit at 12, got %d", got)
	}
}

func TestOffsetTrackerTracksPartitionsIndependently(t *testing.T) {
	tracker := newOffsetTracker("topic")
	tracker.Delivered(0, 1)
	tracker.Delivered(1, 5)

	tracker.Mark(1, 5)
	tracker.Mark(0, 1)

	pending := pendingOffsets(t, tracker)
	if pending[0] != 2 || pending[1] != 6 {
		t.Fatalf("unexpected pending offsets: %v", pending)
	}
}

func TestOffsetTrackerForgetPartitions(t *testing.T) {
	tracker := newOffsetTracker("topic")
	tracker.Delivered(0, 1)
	tracker.Mark(0, 1)

	tracker.Forget([]kafka.TopicPartition{{Partition: 0}})
	if got := pendingOffsets(t, tracker); len(got) != 0 {
		t.Fatalf("expected revoked partition state to be dropped, got %v", got)
	}

	// A late acknowledgement for a revoked partition must not recreate state.
	tracker.Mark(0, 2)
	if got := pendingOffsets(t, tracker); len(got) != 0 {
		t.Fatalf("late ack recreated revoked partition state: %v", got)
	}
}
