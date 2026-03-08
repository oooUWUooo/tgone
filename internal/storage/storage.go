// Package storage manages persistent bot subscription state.
package storage

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Subscription holds a single chat's preferences.
type Subscription struct {
	ChatID    int64     `json:"chat_id"`
	HubIDs    []string  `json:"hub_ids"`
	CreatedAt time.Time `json:"created_at"`
}

// Store is a thread-safe subscription registry that persists to a JSON file.
type Store struct {
	mu   sync.RWMutex
	subs map[int64]*Subscription
	path string
}

// New loads existing subscriptions from path (if present) and returns a ready Store.
// path may be empty, in which case subscriptions are in-memory only.
func New(path string) *Store {
	s := &Store{
		subs: make(map[int64]*Subscription),
		path: path,
	}
	if path == "" {
		return s
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return s // first run — file not yet created
	}
	var loaded []*Subscription
	if err := json.Unmarshal(data, &loaded); err == nil {
		for _, sub := range loaded {
			s.subs[sub.ChatID] = sub
		}
	}
	return s
}

// Subscribe registers chatID with the given hub IDs.
// Calling Subscribe on an already-subscribed chat replaces its preferences.
func (s *Store) Subscribe(chatID int64, hubIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subs[chatID] = &Subscription{
		ChatID:    chatID,
		HubIDs:    hubIDs,
		CreatedAt: time.Now(),
	}
	s.persist()
}

// Unsubscribe removes chatID and returns whether it existed.
func (s *Store) Unsubscribe(chatID int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.subs[chatID]
	if ok {
		delete(s.subs, chatID)
		s.persist()
	}
	return ok
}

// Get returns the subscription for chatID, or nil if not subscribed.
func (s *Store) Get(chatID int64) *Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if sub, ok := s.subs[chatID]; ok {
		cp := *sub
		return &cp
	}
	return nil
}

// All returns a snapshot of all active subscriptions.
func (s *Store) All() []*Subscription {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Subscription, 0, len(s.subs))
	for _, sub := range s.subs {
		cp := *sub
		out = append(out, &cp)
	}
	return out
}

// Count returns the number of active subscriptions.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.subs)
}

// persist writes the current state to the JSON file. Must be called with mu held.
func (s *Store) persist() {
	if s.path == "" {
		return
	}
	list := make([]*Subscription, 0, len(s.subs))
	for _, v := range s.subs {
		list = append(list, v)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.path, data, 0644)
}
