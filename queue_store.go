package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"bili-queue-overlay/internal/queue"
)

type queueSnapshot struct {
	Room    string        `json:"room"`
	Entries []queue.Entry `json:"entries"`
}

type queueStore struct {
	mu         sync.Mutex
	path, room string
	last       []byte
}

func newQueueStore(cfg Config) *queueStore {
	room := fmt.Sprintf("room:%d", cfg.RoomID)
	if cfg.IdentityCode != "" {
		sum := sha256.Sum256([]byte(cfg.IdentityCode))
		room = "identity:" + hex.EncodeToString(sum[:])
	}
	return &queueStore{path: filepath.Join(exeDir(), "queue_state.json"), room: room}
}

func (s *queueStore) load() ([]queue.Entry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snapshot queueSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, err
	}
	if snapshot.Room != s.room {
		return nil, nil
	}
	return snapshot.Entries, nil
}

func (s *queueStore) save(entries []queue.Entry) error {
	return s.saveLatest(func() []queue.Entry { return entries })
}

func (s *queueStore) saveLatest(snapshot func() []queue.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries := snapshot()
	data, err := json.Marshal(queueSnapshot{Room: s.room, Entries: entries})
	if err != nil {
		return err
	}
	if bytes.Equal(data, s.last) {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".queue_state-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(name, s.path); err != nil {
		return err
	}
	s.last = data
	return nil
}
