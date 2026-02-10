package watcher

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

// newTestWatcher creates a Watcher with isolated channels that have no
// background fsnotify goroutine, so tests can send/close channels without
// races.  The caller controls the returned events and errs channels.
func newTestWatcher(t *testing.T) (*Watcher, chan fsnotify.Event, chan error) {
	t.Helper()
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatalf("NewWatcher: %v", err)
	}
	// Stop internal goroutine immediately; channels are now closed.
	_ = fsw.Close()

	// Replace closed channels with fresh ones we control.
	events := make(chan fsnotify.Event)
	errs := make(chan error, 1)
	fsw.Events = events
	fsw.Errors = errs

	return &Watcher{fsw: fsw, callback: func() {}}, events, errs
}

// TestRun_EventsChannelClosed verifies Run exits when the Events channel is
// closed.
func TestRun_EventsChannelClosed(t *testing.T) {
	w, events, _ := newTestWatcher(t)

	done := make(chan struct{})
	go func() {
		w.Run(context.Background(), nil)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	close(events)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Events channel closed")
	}
}

// TestRun_ErrorsChannelClosed verifies Run exits when the Errors channel is
// closed.
func TestRun_ErrorsChannelClosed(t *testing.T) {
	w, _, errs := newTestWatcher(t)

	done := make(chan struct{})
	go func() {
		w.Run(context.Background(), nil)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	close(errs)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not return after Errors channel closed")
	}
}

// TestRun_ErrorCallbackInvoked sends an error on the Errors channel and
// verifies the errFn callback is called.
func TestRun_ErrorCallbackInvoked(t *testing.T) {
	w, _, errs := newTestWatcher(t)

	var gotErrors atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Run(ctx, func(_ error) {
			gotErrors.Add(1)
		})
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	errs <- errors.New("injected test error")
	time.Sleep(50 * time.Millisecond)

	if got := gotErrors.Load(); got != 1 {
		t.Errorf("expected 1 error callback, got %d", got)
	}

	cancel()
	<-done
}

// TestRun_NonMeaningfulOpIgnored verifies that events with non-meaningful
// operations (e.g. CHMOD) do not trigger the callback.
func TestRun_NonMeaningfulOpIgnored(t *testing.T) {
	var called atomic.Int32
	w, events, _ := newTestWatcher(t)
	w.callback = func() { called.Add(1) }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Run(ctx, nil)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	// CHMOD is not in the meaningful ops mask.
	events <- fsnotify.Event{Name: "test.md", Op: fsnotify.Chmod}
	time.Sleep(150 * time.Millisecond)

	if got := called.Load(); got != 0 {
		t.Errorf("CHMOD should not trigger callback, got %d calls", got)
	}

	cancel()
	<-done
}
