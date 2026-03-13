package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/yamux"

	"proxy-bridge/pkg/tunnel"
)

const tunnelResponseTimeout = 15 * time.Second

type tunnelState struct {
	session *yamux.Session
	edgeID  string
	mu      sync.Mutex
}

func (s *Server) getEdgeList() ([]struct {
	EdgeID  string `json:"edge_id"`
	Addr    string `json:"addr"`
	Country string `json:"country"`
}, error) {
	url := s.cfg.BridgeURL + "/api/edges?token=" + s.cfg.Token
	if s.cfg.Country != "" {
		url += "&country=" + s.cfg.Country
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("edges: %d", resp.StatusCode)
	}
	var list []struct {
		EdgeID  string `json:"edge_id"`
		Addr    string `json:"addr"`
		Country string `json:"country"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return nil, err
	}
	return list, nil
}

func (s *Server) authBridge() bool {
	body := `{"token":"` + s.cfg.Token + `"`
	if s.cfg.ID != "" {
		body += `,"client_id":"` + s.cfg.ID + `"`
	}
	body += `}`
	resp, err := http.Post(s.cfg.BridgeURL+"/api/client/auth", "application/json", strings.NewReader(body))
	if err != nil {
		log.Printf("client: auth: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var out struct {
		UserID uint `json:"user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err == nil && out.UserID > 0 {
		s.userID = out.UserID
	}
	return true
}

func (s *Server) ensureTunnel() (sess *yamux.Session, edgeID string, err error) {
	ts := s.getTunnelState()
	ts.mu.Lock()
	if ts.session != nil && ts.edgeID != "" {
		sess, edgeID = ts.session, ts.edgeID
		ts.mu.Unlock()
		return sess, edgeID, nil
	}
	ts.mu.Unlock()

	// 锁外做网络 I/O
	if s.userID == 0 {
		s.authBridge()
	}
	list, err := s.getEdgeList()
	if err != nil || len(list) == 0 {
		if err != nil {
			log.Printf("client: edge list: %v", err)
		}
		return nil, "", fmt.Errorf("no edges")
	}
	addr := s.cfg.BridgeTunnel
	if addr == "" {
		addr = "localhost:8081"
	}
	session, err := tunnel.Dial(addr)
	if err != nil {
		return nil, "", err
	}
	stream, err := session.Open()
	if err != nil {
		session.Close()
		return nil, "", err
	}
	if _, err := stream.Write([]byte("CLIENT " + s.cfg.Token + "\n")); err != nil {
		stream.Close()
		session.Close()
		return nil, "", err
	}
	stream.Close()

	// 拿到锁再存，防止重复建连
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.session != nil && ts.edgeID != "" {
		session.Close()
		return ts.session, ts.edgeID, nil
	}
	ts.session = session
	ts.edgeID = list[0].EdgeID
	return ts.session, ts.edgeID, nil
}

// forwardViaTunnel 经 Bridge 隧道连到 targetAddr，返回与 SOCKS 客户端对接的流（已读掉 OK\n）；失败时清 session 以便重连
func (s *Server) forwardViaTunnel(targetAddr string) (net.Conn, error) {
	sess, edgeID, err := s.ensureTunnel()
	if err != nil {
		return nil, err
	}
	stream, err := sess.Open()
	if err != nil {
		s.clearTunnel()
		return nil, err
	}
	req := "CONNECT " + edgeID + " " + fmt.Sprintf("%d", s.userID) + "\nCONNECT " + targetAddr + "\n"
	if _, err := stream.Write([]byte(req)); err != nil {
		stream.Close()
		s.clearTunnel()
		log.Printf("client: tunnel write %s: %v", targetAddr, err)
		return nil, err
	}
	br := bufio.NewReader(stream)
	if err := stream.SetReadDeadline(time.Now().Add(tunnelResponseTimeout)); err == nil {
		defer stream.SetReadDeadline(time.Time{})
	}
	line, err := br.ReadString('\n')
	if err != nil {
		stream.Close()
		s.clearTunnel()
		log.Printf("client: tunnel read OK %s: %v", targetAddr, err)
		return nil, err
	}
	if !strings.HasPrefix(strings.TrimSpace(line), "OK") {
		stream.Close()
		log.Printf("client: tunnel %s: response not OK: %q", targetAddr, strings.TrimSpace(line))
		return nil, fmt.Errorf("tunnel not ok")
	}
	return &connWithReader{Conn: stream, r: br}, nil
}

func (s *Server) clearTunnel() {
	ts := s.getTunnelState()
	ts.mu.Lock()
	if ts.session != nil {
		ts.session.Close()
		ts.session = nil
		ts.edgeID = ""
	}
	ts.mu.Unlock()
}

type connWithReader struct {
	net.Conn
	r *bufio.Reader
}

func (c *connWithReader) Read(p []byte) (n int, err error) { return c.r.Read(p) }
