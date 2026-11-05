package lsp

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestJobGroupWaitReturnsWhenIdle(t *testing.T) {
	var jobs jobGroup
	if err := jobs.wait(context.Background()); err != nil {
		t.Fatal("idle group reported", err)
	}
}

func TestJobGroupWaitHonorsPreCancelledContext(t *testing.T) {
	var jobs jobGroup
	jobs.start()
	defer jobs.done()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := jobs.wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("pre-cancelled wait returned", err)
	}
}

func TestJobGroupCancelledWaitReturnsWithoutCompletion(t *testing.T) {
	var jobs jobGroup
	jobs.start()
	defer jobs.done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- jobs.wait(ctx) }()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal("cancelled wait returned", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled wait blocked on a pending job")
	}
}

func TestJobGroupWaitCompletesWhenLastJobFinishes(t *testing.T) {
	var jobs jobGroup
	jobs.start()
	jobs.start()
	result := make(chan error, 1)
	go func() { result <- jobs.wait(context.Background()) }()
	jobs.done()
	select {
	case <-result:
		t.Fatal("wait returned while a job was still pending")
	case <-time.After(20 * time.Millisecond):
	}
	jobs.done()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal("completed wait returned", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("wait did not return after the last job finished")
	}
}
