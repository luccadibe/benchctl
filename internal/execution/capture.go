package execution

import (
	"bytes"
	"io"
	"reflect"
	"sync"
)

type captureBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func newCaptureBuffer() *captureBuffer {
	return &captureBuffer{}
}

func (c *captureBuffer) Write(p []byte) (int, error) {
	c.mu.Lock()
	n, err := c.buf.Write(p)
	c.mu.Unlock()
	return n, err
}

func (c *captureBuffer) Bytes() []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]byte(nil), c.buf.Bytes()...)
}

func (c *captureBuffer) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

// multiWriterFiltered filters out duplicate writers by pointer.
// This is to avoid duplicate writes to the same writer.
// For example, when using a pipe and a file writer, the pipe will write to the file writer
// but we don't want to write to the file writer twice. Not the cleanest solution, but it works.
// TODO: find a better solution.
func multiWriterFiltered(writers ...io.Writer) io.Writer {
	filtered := make([]io.Writer, 0, len(writers))
	seenPtrs := map[uintptr]struct{}{}
	for _, w := range writers {
		if w == nil {
			continue
		}
		rv := reflect.ValueOf(w)
		skip := false
		if rv.Kind() == reflect.Pointer || rv.Kind() == reflect.UnsafePointer {
			ptr := rv.Pointer()
			if _, ok := seenPtrs[ptr]; ok {
				skip = true
			} else {
				seenPtrs[ptr] = struct{}{}
			}
		}
		if skip {
			continue
		}
		filtered = append(filtered, w)
	}
	if len(filtered) == 0 {
		return io.Discard
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return io.MultiWriter(filtered...)
}
