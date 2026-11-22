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

func TestCancelWaitingEntry(t *testing.T) {
	m := New(nil, 5)
	m.Enqueue(1, "甲", "", "排队", "", "排队", "", 0, 0)
	m.Enqueue(2, "乙", "", "排队", "", "排队", "", 0, 0)
	if !m.Cancel(1) || len(m.List()) != 1 || m.List()[0].UID != 2 {
		t.Fatal("waiting entry not removed")
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if m.Cancel(2) {
		t.Fatal("in-progress entry must not be cancelled")
	}
	if m.Cancel(99) {
		t.Fatal("unknown viewer must not be cancelled")
	}
}

func TestPauseManualMoveAndClear(t *testing.T) {
	m := New(nil, 5)
	m.SetPaused(true)
	if m.Enqueue(1, "甲", "", "排队", "", "排队", "", 0, 0) {
		t.Fatal("paused queue accepted automatic entry")
	}
	if !m.EnqueueManual(1, "甲", "排队", "") {
		t.Fatal("manual entry should work while paused")
	}
	m.SetPaused(false)
	if !m.Enqueue(2, "乙", "", "排队", "", "排队", "", 0, 0) {
		t.Fatal("resumed queue rejected entry")
	}
	if !m.MoveWaiting(2, -1) || m.List()[0].UID != 2 {
		t.Fatal("move up failed")
	}
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if m.MoveWaiting(1, -1) {
		t.Fatal("must not move ahead of in-progress entry")
	}
	m.Clear()
	if len(m.List()) != 0 || !m.Enqueue(1, "甲", "", "排队", "", "排队", "", 0, 0) {
		t.Fatal("clear did not reset queue index")
	}
}
