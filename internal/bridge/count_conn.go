package bridge

import (
	"io"
	"net"
	"sync"
)

// countConn 包装 net.Conn 并统计读写字节数
type countConn struct {
	net.Conn
	in  *int64
	out *int64
	mu  *sync.Mutex
}

func (c *countConn) Read(p []byte) (n int, err error) {
	n, err = c.Conn.Read(p)
	if n > 0 && c.in != nil {
		c.mu.Lock()
		*c.in += int64(n)
		c.mu.Unlock()
	}
	return n, err
}

func (c *countConn) Write(p []byte) (n int, err error) {
	n, err = c.Conn.Write(p)
	if n > 0 && c.out != nil {
		c.mu.Lock()
		*c.out += int64(n)
		c.mu.Unlock()
	}
	return n, err
}

// countCopy 从 src 复制到 dst 并统计：in 为写到 dst 的字节（对 dst 而言是 in），out 为从 src 读的字节
func countCopy(dst net.Conn, src io.Reader, in, out *int64, mu *sync.Mutex) (int64, error) {
	w := &countWriter{dst: dst, out: out, mu: mu}
	return io.Copy(w, src)
}

type countWriter struct {
	dst net.Conn
	out *int64
	mu  *sync.Mutex
}

func (c *countWriter) Write(p []byte) (n int, err error) {
	n, err = c.dst.Write(p)
	if n > 0 && c.out != nil {
		c.mu.Lock()
		*c.out += int64(n)
		c.mu.Unlock()
	}
	return n, err
}
