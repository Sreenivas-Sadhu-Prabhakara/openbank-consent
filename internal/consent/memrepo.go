package consent

import (
	"context"
	"sync"
)

// MemRepository is an in-memory Repository used by unit and handler tests and
// for running the service without a database. It is safe for concurrent use.
type MemRepository struct {
	mu    sync.RWMutex
	store map[string]Consent
}

// NewMemRepository returns an empty in-memory repository.
func NewMemRepository() *MemRepository {
	return &MemRepository{store: make(map[string]Consent)}
}

func (r *MemRepository) Create(_ context.Context, c *Consent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[c.ID] = *c // store a copy so external mutation cannot corrupt state
	return nil
}

func (r *MemRepository) Get(_ context.Context, id string) (*Consent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.store[id]
	if !ok {
		return nil, ErrNotFound
	}
	out := c
	return &out, nil
}

func (r *MemRepository) Update(_ context.Context, c *Consent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.store[c.ID]; !ok {
		return ErrNotFound
	}
	r.store[c.ID] = *c
	return nil
}

func (r *MemRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.store[id]; !ok {
		return ErrNotFound
	}
	delete(r.store, id)
	return nil
}
