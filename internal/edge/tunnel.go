package edge

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"proxy-bridge/pkg/tunnel"
)

// runTunnel 连接 Bridge 隧道，发送 EDGE id，然后处理 Bridge 转发的 CONNECT 流
func (s *Server) runTunnel() {
	addr := s.cfg.BridgeTunnel
	if addr == "" {
		addr = "localhost:8081"
	}
	session, err := tunnel.Dial(addr)
	if err != nil {
		log.Printf("edge: tunnel dial %s: %v", addr, err)
		return
	}
	defer session.Close()
	stream, err := session.Open()
	if err != nil {
		log.Printf("edge: open stream: %v", err)
		return
	}
	_, err = stream.Write([]byte("EDGE " + s.cfg.ID + "\n"))
	stream.Close()
	if err != nil {
		return
	}
	// 隧道注册成功，不打印 info 日志
	for {
		stream, err := session.Accept()
		if err != nil {
			return
		}
		go s.handleConnectStream(stream)
	}
}

var (
	connLimitMu sync.Mutex
	connSem     chan struct{}
)

func (s *Server) allowConn() bool {
	max := s.cfg.MaxConnections
	if max <= 0 {
		max = 1000
	}
	connLimitMu.Lock()
	if connSem == nil || cap(connSem) != max {
		connSem = make(chan struct{}, max)
	}
	connLimitMu.Unlock()
	select {
	case connSem <- struct{}{}:
		return true
	default:
		return false
	}
}

func (s *Server) releaseConn() {
	connLimitMu.Lock()
	sem := connSem
	connLimitMu.Unlock()
	if sem != nil {
		<-sem
	}
}

// handleConnectStream 读 CONNECT host:port，连目标并双向转发，先回 OK\n
func (s *Server) handleConnectStream(stream net.Conn) {
	defer stream.Close()
	if !s.allowConn() {
		return
	}
	defer s.releaseConn()
	br := bufio.NewReader(stream)
	line, err := br.ReadString('\n')
	if err != nil {
		return
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "CONNECT ") {
		return
	}
	target := strings.TrimSpace(strings.TrimPrefix(line, "CONNECT "))
	if target == "" {
		return
	}
	targetConn, err := net.Dial("tcp", target)
	if err != nil {
		log.Printf("edge: connect %s: %v", target, err)
		return
	}
	defer targetConn.Close()
	if _, err := stream.Write([]byte("OK\n")); err != nil {
		return
	}
	if n := br.Buffered(); n > 0 {
		buf := make([]byte, n)
		nr, _ := br.Read(buf)
		if nr > 0 {
			if _, err := targetConn.Write(buf[:nr]); err != nil {
				return
			}
		}
	}
	done := make(chan struct{})
	go func() {
		io.Copy(targetConn, stream)
		targetConn.Close()
		done <- struct{}{}
	}()
	io.Copy(stream, targetConn)
	stream.Close()
	<-done
}
