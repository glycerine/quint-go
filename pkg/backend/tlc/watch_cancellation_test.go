package tlc

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	native "github.com/glycerine/tlago/tlc"
)

// stopRecorder implements native.Checker by embedding the interface; only
// Stop is called by watchCancellation, so the other methods are never reached.
type stopRecorder struct {
	native.Checker
	stops atomic.Int32
}

func (r *stopRecorder) Stop() { r.stops.Add(1) }

// The main checker is process-global, so these tests must not run in parallel
// with each other or with other tests that publish a checker.
func TestWatchCancellationStopsCheckerPublishedAfterCancel(t *testing.T) {
	native.SetMainChecker(nil)
	defer native.SetMainChecker(nil)
	ctx, cancel := context.WithCancel(context.Background())
	stop := watchCancellation(ctx)
	defer stop()

	// Cancellation arrives before the checker is published; the watcher must
	// keep polling until the checker appears and then stop it exactly once.
	cancel()
	time.Sleep(5 * time.Millisecond)
	recorder := &stopRecorder{}
	native.SetMainChecker(recorder)
	deadline := time.Now().Add(5 * time.Second)
	for recorder.stops.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := recorder.stops.Load(); got != 1 {
		t.Fatal("late-published checker was not stopped after cancellation", got)
	}
	stop()
	stop()
	if got := recorder.stops.Load(); got != 1 {
		t.Fatal("checker stopped more than once", got)
	}
}

func TestWatchCancellationIdleStopDoesNotStopChecker(t *testing.T) {
	native.SetMainChecker(nil)
	defer native.SetMainChecker(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	recorder := &stopRecorder{}
	native.SetMainChecker(recorder)
	stop := watchCancellation(ctx)
	// Stop joins the watcher goroutine; with no cancellation it must return
	// without touching the checker that the run still owns.
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not join after normal completion")
	}
	if got := recorder.stops.Load(); got != 0 {
		t.Fatal("uncancelled run stopped the checker", got)
	}
}

func TestWatchCancellationStopsActiveCheckerOnCancel(t *testing.T) {
	native.SetMainChecker(nil)
	defer native.SetMainChecker(nil)
	ctx, cancel := context.WithCancel(context.Background())
	recorder := &stopRecorder{}
	native.SetMainChecker(recorder)
	stop := watchCancellation(ctx)
	defer stop()
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for recorder.stops.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := recorder.stops.Load(); got != 1 {
		t.Fatal("active checker was not stopped on cancellation", got)
	}
}
