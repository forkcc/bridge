package bridge

import (
	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"sync"

	"github.com/hashicorp/yamux"

	"proxy-bridge/pkg/tunnel"
)

// relay 隧道转发：Edge 与 Client 连接管理、双向转发（TCP，QUIC 后续在 Client/Edge 侧加）
type relay struct {
	cfg       *Config
	edgeConns map[string]*yamux.Session
	mu        sync.RWMutex
}

func newRelay(cfg *Config) *relay {
	return &relay{cfg: cfg, edgeConns: make(map[string]*yamux.Session)}
}

func (r *relay) run() error {
	addr := r.cfg.EdgeListen
	if addr == "" {
		addr = ":8081"
	}
	ln, err := tunnel.Listen(addr)
	if err != nil {
		return err
	}
	defer ln.Close()
	server := tunnel.NewServer(ln)
	log.Printf("bridge: tunnel listening on %s (TCP)", addr)
	for {
		session, err := server.Accept()
		if err != nil {
			return err
		}
		go r.handleSession(session)
	}
}

func (r *relay) handleSession(session *yamux.Session) {
	defer session.Close()
	stream, err := session.Accept()
	if err != nil {
		return
	}
	line, err := bufio.NewReader(stream).ReadString('\n')
	if err != nil {
		stream.Close()
		return
	}
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "EDGE ") {
		edgeID := strings.TrimSpace(strings.TrimPrefix(line, "EDGE "))
		stream.Close()
		r.mu.Lock()
		if old, ok := r.edgeConns[edgeID]; ok {
			old.Close()
		}
		r.edgeConns[edgeID] = session
		r.mu.Unlock()
		log.Printf("bridge: edge %s connected", edgeID)
		return
	}
	if strings.HasPrefix(line, "CLIENT ") {
		stream.Close()
		r.handleClientSession(session)
		return
	}
	stream.Close()
}

func (r *relay) handleClientSession(session *yamux.Session) {
	for {
		stream, err := session.Accept()
		if err != nil {
			return
		}
		go r.handleClientStream(stream)
	}
}

func (r *relay) handleClientStream(stream net.Conn) {
	defer stream.Close()
	br := bufio.NewReader(stream)
	line, err := br.ReadString('\n')
	if err != nil {
		return
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "CONNECT ") {
		return
	}
	edgeID := strings.TrimSpace(strings.TrimPrefix(line, "CONNECT "))
	r.mu.RLock()
	edgeSession, ok := r.edgeConns[edgeID]
	r.mu.RUnlock()
	if !ok {
		return
	}
	edgeStream, err := edgeSession.Open()
	if err != nil {
		return
	}
	defer edgeStream.Close()
	if br.Buffered() > 0 {
		_, _ = io.Copy(edgeStream, br)
	}
	go io.Copy(edgeStream, stream)
	io.Copy(stream, edgeStream)
}
