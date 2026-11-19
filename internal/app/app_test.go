package app

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"sync"
	"testing"
	"time"
)

// Stopping goes in order: the servers first, so no request starts anything
// new; then the workers; then whatever the requests started is waited for;
// and the database and Redis are let go of last, once nothing uses them.
func TestStoppingGoesInOrder(t *testing.T) {
	var mu sync.Mutex
	var order []string
	note := func(step string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, step)
	}

	// A request still under way when the stop comes: the server's shutdown
	// waits for it, so its end is noted before anything after the servers.
	started := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		time.Sleep(50 * time.Millisecond)
		note("request finished")
	})
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := free.Addr().String()
	free.Close()
	server := &http.Server{Addr: addr, Handler: handler}

	ctx, stop := context.WithCancel(context.Background())
	go func() {
		// Ask until the server is listening, then stop while it answers.
		for {
			if _, err := http.Get("http://" + addr); err == nil {
				return
			}
			select {
			case <-started:
				return
			case <-time.After(10 * time.Millisecond):
			}
		}
	}()
	go func() {
		<-started
		stop()
	}()

	worker := func(ctx context.Context) {
		<-ctx.Done()
		note("worker stopped")
	}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	err = runInOrder(ctx, log, map[string]*http.Server{"test": server}, []func(context.Context){worker},
		func(context.Context) { note("drained") },
		func() { note("closed") })
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"request finished", "worker stopped", "drained", "closed"}
	if !slices.Equal(order, want) {
		t.Errorf("stopped in the order %v, want %v", order, want)
	}
}
