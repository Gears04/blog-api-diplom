package logger

import (
	"errors"
	"os"
	"sync"
	"time"
)

const writeDelay = time.Second

type message struct {
	text    string
	writeAt time.Time
}

// Logger writes queued messages to a file in one background goroutine.
type Logger struct {
	mu       sync.RWMutex
	closed   bool
	messages chan message
	done     chan struct{}
	file     *os.File
	err      error
}

func New(path string) (*Logger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	l := &Logger{
		messages: make(chan message, 256),
		done:     make(chan struct{}),
		file:     file,
	}
	go l.run()
	return l, nil
}

// Write copies the message because log.Logger can reuse its byte buffer.
// A full queue waits for space instead of silently losing messages.
func (l *Logger) Write(data []byte) (int, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.closed {
		return 0, os.ErrClosed
	}
	l.messages <- message{text: string(data), writeAt: time.Now().Add(writeDelay)}
	return len(data), nil
}

func (l *Logger) run() {
	defer close(l.done)
	for message := range l.messages {
		// Delay is counted from enqueue time, not from the previous file write.
		// Close drains the same queue without cancelling this delay.
		time.Sleep(time.Until(message.writeAt))
		if _, err := l.file.WriteString(message.text); err != nil && l.err == nil {
			l.err = err
		}
	}
	l.err = errors.Join(l.err, l.file.Close())
}

// Close waits until all accepted messages are written before closing the file.
// It is safe to call Close more than once, including from different goroutines.
func (l *Logger) Close() error {
	l.mu.Lock()
	if !l.closed {
		l.closed = true
		close(l.messages)
	}
	l.mu.Unlock()
	<-l.done
	return l.err
}
