package main

import "sync"

// Registry bounds concurrent sessions and waits for their cleanup at shutdown.
type Registry struct {
	mu       sync.Mutex
	wg       sync.WaitGroup
	active   int
	limit    int
	stopping bool
}

func newRegistry(limit int) *Registry { return &Registry{limit: limit} }

func (r *Registry) acquire() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopping || r.active >= r.limit {
		return false
	}
	r.active++
	r.wg.Add(1)
	return true
}

func (r *Registry) release() {
	r.mu.Lock()
	r.active--
	r.mu.Unlock()
	r.wg.Done()
}

func (r *Registry) stop() {
	r.mu.Lock()
	r.stopping = true
	r.mu.Unlock()
}

func (r *Registry) wait() { r.wg.Wait() }
