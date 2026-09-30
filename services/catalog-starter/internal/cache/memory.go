package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"
)

type entry struct {
	data    []byte
	expires time.Time
}

type Memory struct {
	ttl     time.Duration
	mu      sync.Mutex
	entries map[string]entry
}

func NewMemory(ttl time.Duration) *Memory {
	return &Memory{ttl: ttl, entries: map[string]entry{}}
}

func (m *Memory) Get(_ context.Context, key string, dst any) (bool, error) {
	m.mu.Lock()
	e, ok := m.entries[key]
	m.mu.Unlock()
	if !ok || time.Now().After(e.expires) {
		return false, nil
	}
	return true, json.Unmarshal(e.data, dst)
}

func (m *Memory) Set(_ context.Context, key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.entries[key] = entry{data: data, expires: time.Now().Add(m.ttl)}
	m.mu.Unlock()
	return nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.entries, key)
	m.mu.Unlock()
	return nil
}
