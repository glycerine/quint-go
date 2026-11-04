package lsp

import (
	"context"
	"sync"
)

// jobGroup gives all waiters one completion channel per active batch. Its
// mutex is independent of document publication, which may be blocked in caller
// output. A cancelled wait never creates a goroutine waiting on another worker.
type jobGroup struct {
	mu      sync.Mutex
	pending int
	idle    chan struct{}
}

func (jobs *jobGroup) start() {
	jobs.mu.Lock()
	if jobs.pending == 0 {
		jobs.idle = make(chan struct{})
	}
	jobs.pending++
	jobs.mu.Unlock()
}

func (jobs *jobGroup) done() {
	jobs.mu.Lock()
	jobs.pending--
	if jobs.pending == 0 {
		close(jobs.idle)
	}
	jobs.mu.Unlock()
}

func (jobs *jobGroup) wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	jobs.mu.Lock()
	done := jobs.idle
	pending := jobs.pending
	jobs.mu.Unlock()
	if pending == 0 {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return ctx.Err()
	}
}
