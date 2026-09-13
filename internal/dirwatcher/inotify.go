package dirwatcher

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// WatchForNewFiles Depending on the system, a single "write" can generate many Write events; for
// example compiling a large Go program can generate hundreds of Write events on
// the binary.
//
// The general strategy to deal with this is to wait a short time for more write
// events, resetting the wait period for every new event.
func WatchForNewFiles(ctx context.Context, paths ...string) (<-chan fsnotify.Event, error) {
	if len(paths) < 1 {
		return nil, errors.New("must specify at least one path to watch")
	}

	// Create a new watcher.
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("creating a new watcher: %v", err)
	}

	// Do not specify channel direction in make as it will also be limited at its eventual call site
	events := make(chan fsnotify.Event)

	// Start listening for events.
	go dedupLoop(ctx, w, events)
	fmt.Println("dir watcher started...")

	// Add all paths from the commandline.
	for _, p := range paths {
		err = w.Add(p)
		if err != nil {
			return nil, fmt.Errorf("%q: %v", p, err)
		}
		fmt.Printf("added dir %s to watchlist\n", p)
	}

	fmt.Println("all paths added to watchlist...")

	return events, nil
}

func dedupLoop(ctx context.Context, w *fsnotify.Watcher, events chan<- fsnotify.Event) {
	defer w.Close()
	defer close(events)

	var (
		// Wait 100ms for new events; each new event resets the timer.
		waitFor = 100 * time.Millisecond

		// Keep track of the timers, as path → timer.
		mu     sync.Mutex
		timers = make(map[string]*time.Timer)

		// Callback we run.
		printEvent = func(e fsnotify.Event) {
			printTime(e.String())

			// Don't need to remove the timer if you don't have a lot of files.
			mu.Lock()
			delete(timers, e.Name)
			mu.Unlock()
		}
	)

	for {
		select {
		case <-ctx.Done():
			return
		// Read from Errors.
		case err, ok := <-w.Errors:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}
			printTime("ERROR: %s", err)
		// Read from Events.
		case e, ok := <-w.Events:
			if !ok { // Channel was closed (i.e. Watcher.Close() was called).
				return
			}

			// We just want to watch for file creation, so ignore everything
			// outside of Create and Write.
			if !e.Has(fsnotify.Create) && !e.Has(fsnotify.Write) {
				continue
			}

			// Only watch for torrent files
			if filepath.Ext(e.Name) != ".torrent" {
				continue
			}

			// Get timer.
			mu.Lock()
			t, ok := timers[e.Name]
			mu.Unlock()

			// No timer yet, so create one.
			if !ok {
				t = time.AfterFunc(math.MaxInt64, func() {
					printEvent(e)
					events <- e
				})
				t.Stop()

				mu.Lock()
				timers[e.Name] = t
				mu.Unlock()
			}

			// Reset the timer for this path, so it will start from 100ms again.
			t.Reset(waitFor)
		}
	}
}

func printTime(s string, args ...any) {
	fmt.Printf(time.Now().Format("15:04:05.0000")+" "+s+"\n", args...)
}

func exit(format string, a ...any) {
	fmt.Fprintf(os.Stderr, filepath.Base(os.Args[0])+": "+format+"\n", a...)
	os.Exit(1)
}
