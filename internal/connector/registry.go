package connector

import (
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Connector{}
)

// Register mendaftarkan connector. Panic bila ID kosong atau duplikat.
func Register(c Connector) {
	id := c.ID()
	if id == "" {
		panic("connector: ID kosong")
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registry[id]; ok {
		panic("connector: ID duplikat: " + id)
	}
	registry[id] = c
}

// List mengembalikan salinan seluruh connector terurut menurut ID.
func List() []Connector {
	mu.RLock()
	out := make([]Connector, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID() < out[j].ID() })
	return out
}

// Get mencari connector menurut ID.
func Get(id string) (Connector, bool) {
	mu.RLock()
	defer mu.RUnlock()
	c, ok := registry[id]
	return c, ok
}
