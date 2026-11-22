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

func TestQueueStoreLegacyMigrationAndRoomFiles(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "queue_state.json")
	old := &queueStore{path: legacy, room: "room:1"}
	entries := []queue.Entry{{UID: 123, Username: "甲", JoinedAt: time.Now(), Status: queue.StatusActive}}
	if err := old.save(entries); err != nil {
		t.Fatal(err)
	}
	a := &queueStore{path: filepath.Join(dir, "room-a.json"), legacyPath: legacy, room: "room:1"}
	b := &queueStore{path: filepath.Join(dir, "room-b.json"), legacyPath: legacy, room: "room:2"}
	got, err := a.load()
	if err != nil || len(got) != 1 {
		t.Fatalf("legacy restore failed: %+v, %v", got, err)
	}
	if got, err := b.load(); err != nil || len(got) != 0 {
		t.Fatal("legacy queue leaked to other room")
	}
	if err := a.save(entries); err != nil {
		t.Fatal(err)
	}
	if err := b.save([]queue.Entry{{UID: 456, JoinedAt: time.Now()}}); err != nil {
		t.Fatal(err)
	}
	got, err = a.load()
	if err != nil || len(got) != 1 || got[0].UID != 123 {
		t.Fatal("other room overwrote saved queue")
	}
}
