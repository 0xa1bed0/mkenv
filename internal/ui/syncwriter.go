package ui

import (
	"io"
	"os"
	"sync"
	"time"
)

// SyncWriter wraps an *os.File and periodically syncs to disk.
// This ensures data is visible to bind-mounted readers (e.g., containers)
// without the overhead of syncing after every write.
type SyncWriter struct {
	f        *os.File
	mu       sync.Mutex
	dirty    bool
	stopCh   chan struct{}
	doneCh   chan struct{}
	interval time.Duration
}

// NewSyncWriter creates a new SyncWriter that syncs at the given interval.
// A typical interval is 100-500ms for balancing visibility and performance.
func NewSyncWriter(f *os.File, interval time.Duration) *SyncWriter {
	if interval <= 0 {
		interval = 200 * time.Millisecond
	}
	sw := &SyncWriter{
		f:        f,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
		interval: interval,
	}
	go sw.syncLoop()
	return sw
}

func (sw *SyncWriter) syncLoop() {
	defer close(sw.doneCh)
	ticker := time.NewTicker(sw.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sw.mu.Lock()
			if sw.dirty {
				sw.f.Sync()
				sw.dirty = false
			}
			sw.mu.Unlock()
		case <-sw.stopCh:
			// Final sync before exit
			sw.mu.Lock()
			if sw.dirty {
				sw.f.Sync()
				sw.dirty = false
			}
			sw.mu.Unlock()
			return
		}
	}
}

func (sw *SyncWriter) Write(p []byte) (int, error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	n, err := sw.f.Write(p)
	if n > 0 {
		sw.dirty = true
	}
	return n, err
}

// Sync forces an immediate sync to disk.
func (sw *SyncWriter) Sync() error {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.dirty {
		err := sw.f.Sync()
		sw.dirty = false
		return err
	}
	return nil
}

// Close stops the sync loop and closes the underlying file.
func (sw *SyncWriter) Close() error {
	close(sw.stopCh)
	<-sw.doneCh // Wait for sync loop to finish
	return sw.f.Close()
}

// Ensure SyncWriter implements io.WriteCloser
var _ io.WriteCloser = (*SyncWriter)(nil)

// TimestampWriter wraps an io.Writer and prepends a timestamp to each write.
// This is used to add timestamps at the final log destination.
type TimestampWriter struct {
	w io.Writer
}

// NewTimestampWriter creates a new TimestampWriter that wraps the given writer.
func NewTimestampWriter(w io.Writer) *TimestampWriter {
	return &TimestampWriter{w: w}
}

func (tw *TimestampWriter) Write(p []byte) (int, error) {
	timestamp := time.Now().Format("2006-01-02T15:04:05.000")
	// Prepend timestamp to the line
	prefixed := "[" + timestamp + "] " + string(p)
	n, err := tw.w.Write([]byte(prefixed))
	if err != nil {
		return 0, err
	}
	// Return original length since caller expects that
	if n > 0 {
		return len(p), nil
	}
	return 0, nil
}

// Sync forwards sync to underlying writer if it supports it.
func (tw *TimestampWriter) Sync() error {
	if s, ok := tw.w.(syncer); ok {
		return s.Sync()
	}
	return nil
}

// Close forwards close to underlying writer if it supports it.
func (tw *TimestampWriter) Close() error {
	if c, ok := tw.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
