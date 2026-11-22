package main

import (
	"bili-queue-overlay/internal/queue"
	"path/filepath"
	"testing"
	"time"
)

func TestQueueStoreRoundTripAndRoomIsolation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.json")
	s := &queueStore{path: path, room: "room:1"}
	want := []queue.Entry{{UID: 123, Username: "观众", JoinedAt: time.Now(), Status: queue.StatusActive}}
	if err := s.save(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.load()
	if err != nil || len(got) != 1 || got[0].UID != 123 {
		t.Fatalf("restore: %+v, %v", got, err)
	}
	other := &queueStore{path: path, room: "room:2"}
	got, err = other.load()
	if err != nil || len(got) != 0 {
		t.Fatalf("other room restored entries: %+v, %v", got, err)
	}
	if err := s.save(nil); err != nil {
		t.Fatal(err)
	}
	got, err = s.load()
	if err != nil || len(got) != 0 {
		t.Fatalf("clear: %+v, %v", got, err)
	}
}
