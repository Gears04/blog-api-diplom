package logger

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoggerDelaysAndDrainsBurstOnClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.txt")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	started := time.Now()
	if _, err := l.Write([]byte("first\n")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	data, err := os.ReadFile(path)
	if err != nil || len(data) != 0 {
		t.Fatalf("message written before the delay: %q, %v", data, err)
	}
	lastEnqueued := time.Now()
	for i := 0; i < 20; i++ {
		if _, err := l.Write([]byte("event\n")); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(lastEnqueued) < time.Second {
		t.Fatal("Close skipped the delay")
	}
	if elapsed := time.Since(started); elapsed > 3*time.Second {
		t.Fatalf("burst took %v; delay must not accumulate per message", elapsed)
	}
	data, err = os.ReadFile(path)
	if err != nil || strings.Count(string(data), "\n") != 21 {
		t.Fatalf("Close lost events: %v", err)
	}
}

func TestLoggerCreatesFileCopiesMessagesAndAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.txt")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("log.txt was not created: %v", err)
	}
	message := []byte("original\n")
	if _, err := l.Write(message); err != nil {
		t.Fatal(err)
	}
	copy(message, "modified\n")
	for i := 0; i < 1000; i++ {
		if _, err := fmt.Fprintf(l, "message %d\n", i); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Write([]byte("after close\n")); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("write after Close returned %v", err)
	}
	second, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := second.Write([]byte("after restart\n"))
	closeErr := second.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("append errors: %v, %v", writeErr, closeErr)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if strings.Count(text, "\n") != 1002 || !strings.HasPrefix(text, "original\n") || !strings.HasSuffix(text, "after restart\n") {
		t.Fatal("messages were changed, lost, or overwritten")
	}
}

func TestLoggerWritesAllConcurrentMessages(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.txt")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	var writers sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for i := 0; i < 300; i++ {
				if _, err := l.Write([]byte("entry\n")); err != nil {
					t.Errorf("write: %v", err)
				}
			}
		}()
	}
	writers.Wait()
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "\n"); got != 2400 {
		t.Fatalf("wrote %d messages, want 2400", got)
	}
}

func TestLoggerConcurrentWritesAndClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.txt")
	l, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	var writers sync.WaitGroup
	var accepted atomic.Int64
	for worker := 0; worker < 8; worker++ {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for i := 0; i < 300; i++ {
				if _, err := l.Write([]byte("entry\n")); err == nil {
					accepted.Add(1)
				} else if !errors.Is(err, os.ErrClosed) {
					t.Errorf("write: %v", err)
				}
			}
		}()
	}
	var closers sync.WaitGroup
	for i := 0; i < 3; i++ {
		closers.Add(1)
		go func() {
			defer closers.Done()
			if err := l.Close(); err != nil {
				t.Errorf("close: %v", err)
			}
		}()
	}
	writers.Wait()
	closers.Wait()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := int64(strings.Count(string(data), "\n")); got != accepted.Load() {
		t.Fatalf("wrote %d of %d accepted messages", got, accepted.Load())
	}
}

func TestLoggerRejectsInvalidPath(t *testing.T) {
	if l, err := New(t.TempDir()); err == nil {
		_ = l.Close()
		t.Fatal("expected an error for a directory instead of a file")
	}
}
