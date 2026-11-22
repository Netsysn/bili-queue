package queue

import (
	"testing"
	"time"
)

func TestRestoreResetsTimeout(t *testing.T) {
	m := New(nil, 1)
	m.Enqueue(1, "观众", "", "排队", "", "排队", "", 0, 0)
	m.items[0].JoinedAt = time.Now().Add(-2 * time.Minute)
	m.checkTimeout()
	if m.items[0].Status != StatusTimeout {
		t.Fatal("entry should time out")
	}
	m.Restore(0)
	m.checkTimeout()
	if m.items[0].Status != StatusActive {
		t.Fatal("restored entry timed out immediately")
	}
}
