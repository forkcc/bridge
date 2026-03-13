package client

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
)

var (
	totalBytesDown int64
	totalBytesUp   int64
)

func formatSpeed(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1024*1024:
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	case bytesPerSec >= 1024:
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
}

type countReader struct {
	r       io.Reader
	counter *int64
}

func (c *countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		atomic.AddInt64(c.counter, int64(n))
	}
	return n, err
}

type countWriter struct {
	w       io.Writer
	counter *int64
}

func (c *countWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	if n > 0 {
		atomic.AddInt64(c.counter, int64(n))
	}
	return n, err
}

func copyWithCount(dst net.Conn, src net.Conn, counter *int64) {
	io.Copy(&countWriter{w: dst, counter: counter}, src)
}
