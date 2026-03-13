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

	"github.com/hashicorp/yamux"

	"proxy-bridge/pkg/tunnel"
)

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
	body := `{"token":"` + s.cfg.Token + `"}`
	resp, err := http.Post(s.cfg.BridgeURL+"/api/client/auth", "application/json", strings.NewReader(body))
	if err != nil {
		log.Printf("client: auth: %v", err)
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (s *Server) ensureTunnel() (sess *yamux.Session, edgeID string, err error) {
	ts := s.tunnelState
	if ts == nil {
		ts = &tunnelState{}
		s.tunnelState = ts
	}
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.session != nil && ts.edgeID != "" {
		return ts.session, ts.edgeID, nil
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
	list, err := s.getEdgeList()
	if err != nil || len(list) == 0 {
		if err != nil {
			log.Printf("client: edge list: %v", err)
		}
		session.Close()
		return nil, "", fmt.Errorf("no edges")
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
	req := "CONNECT " + edgeID + " 0\nCONNECT " + targetAddr + "\n"
	if _, err := stream.Write([]byte(req)); err != nil {
		stream.Close()
		s.clearTunnel()
		return nil, err
	}
	br := bufio.NewReader(stream)
	line, err := br.ReadString('\n')
	if err != nil {
		stream.Close()
		s.clearTunnel()
		return nil, err
	}
	if !strings.HasPrefix(strings.TrimSpace(line), "OK") {
		stream.Close()
		return nil, fmt.Errorf("tunnel not ok")
	}
	return &connWithReader{Conn: stream, r: br}, nil
}

func (s *Server) clearTunnel() {
	if s.tunnelState != nil {
		s.tunnelState.mu.Lock()
		if s.tunnelState.session != nil {
			s.tunnelState.session.Close()
			s.tunnelState.session = nil
			s.tunnelState.edgeID = ""
		}
		s.tunnelState.mu.Unlock()
	}
}

type connWithReader struct {
	net.Conn
	r *bufio.Reader
}

func (c *connWithReader) Read(p []byte) (n int, err error) { return c.r.Read(p) }
