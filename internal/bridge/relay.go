package bridge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/lionsoul2014/ip2region/binding/golang/service"

	"proxy-bridge/pkg/tunnel"
)

const balanceCacheTTL = 60 * time.Second

type balanceEntry struct {
	balance  int64
	expireAt time.Time
}

const countryUnknown = "unknown"

func (r *relay) countryFromAddr(addr net.Addr) string {
	s := addr.String()
	host, _, _ := net.SplitHostPort(s)
	if host == "" {
		host = s
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "127.0.0.1", "::1", "localhost":
		return "cn"
	}
	if r.ip2r == nil {
		return countryUnknown
	}
	regionStr, err := r.ip2r.SearchByStr(host)
	if err != nil || regionStr == "" {
		return countryUnknown
	}
	parts := strings.Split(regionStr, "|")
	if len(parts) >= 5 && parts[4] != "" {
		return strings.ToLower(strings.TrimSpace(parts[4]))
	}
	return countryUnknown
}

func (r *relay) reportEdgeCountry(edgeID, country string) {
	apiURL := r.cfg.ApihubURL
	if apiURL == "" {
		apiURL = "http://localhost:8082"
	}
	if country == "" {
		country = countryUnknown
	}
	body, _ := json.Marshal(map[string]string{"edge_id": edgeID, "country": country})
	resp, err := http.Post(apiURL+"/api/edge/country", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("bridge: report edge country: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Printf("bridge: report edge country: %d", resp.StatusCode)
		return
	}
	log.Printf("bridge: edge %s country reported: %s", edgeID, country)
}

func (r *relay) reportClientCountry(token, country string) {
	apiURL := r.cfg.ApihubURL
	if apiURL == "" {
		apiURL = "http://localhost:8082"
	}
	if country == "" {
		country = countryUnknown
	}
	body, _ := json.Marshal(map[string]string{"token": token, "country": country})
	resp, err := http.Post(apiURL+"/api/client/country", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("bridge: report client country: %v", err)
		return
	}
	resp.Body.Close()
}

type relay struct {
	cfg       *Config
	ip2r      *service.Ip2Region
	edgeConns map[string]*yamux.Session
	mu        sync.RWMutex

	balCache   map[uint]*balanceEntry
	balCacheMu sync.RWMutex
}

func newRelay(cfg *Config, ip2r *service.Ip2Region) *relay {
	return &relay{
		cfg:       cfg,
		ip2r:      ip2r,
		edgeConns: make(map[string]*yamux.Session),
		balCache:  make(map[uint]*balanceEntry),
	}
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
		session, remoteAddr, err := server.Accept()
		if err != nil {
			return err
		}
		go r.handleSession(session, remoteAddr)
	}
}

func (r *relay) handleSession(session *yamux.Session, remoteAddr net.Addr) {
	stream, err := session.Accept()
	if err != nil {
		return
	}
	line, err := bufio.NewReader(stream).ReadString('\n')
	if err != nil {
		stream.Close()
		session.Close()
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
		country := r.countryFromAddr(remoteAddr)
		r.reportEdgeCountry(edgeID, country)
		log.Printf("bridge: edge %s connected (country=%s)", edgeID, country)
		return
	}
	if strings.HasPrefix(line, "CLIENT ") {
		clientToken := strings.TrimSpace(strings.TrimPrefix(line, "CLIENT "))
		stream.Close()
		country := r.countryFromAddr(remoteAddr)
		r.reportClientCountry(clientToken, country)
		r.handleClientSession(session)
		session.Close()
		return
	}
	stream.Close()
	session.Close()
}

func (r *relay) handleClientSession(session *yamux.Session) {
	for {
		stream, err := session.Accept()
		if err != nil {
			break
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
	parts := strings.Fields(strings.TrimPrefix(line, "CONNECT "))
	if len(parts) < 1 {
		return
	}
	edgeID := parts[0]
	var userID uint
	if len(parts) >= 2 {
		if u, _ := strconv.ParseUint(parts[1], 10, 32); u > 0 {
			userID = uint(u)
		}
	}
	if userID > 0 && r.cfg.ApihubURL != "" && !r.checkBalance(userID) {
		stream.Write([]byte("ERR insufficient balance\n"))
		return
	}
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
	if n := br.Buffered(); n > 0 {
		buf := make([]byte, n)
		nr, _ := br.Read(buf)
		if nr > 0 {
			if _, err := edgeStream.Write(buf[:nr]); err != nil {
				return
			}
		}
	}
	var upBytes, downBytes int64
	var mu sync.Mutex
	wrapped := &countConn{Conn: stream, in: &upBytes, out: &downBytes, mu: &mu}
	done := make(chan struct{})
	go func() {
		io.Copy(edgeStream, wrapped)
		edgeStream.Close()
		done <- struct{}{}
	}()
	io.Copy(wrapped, edgeStream)
	stream.Close()
	<-done

	totalBytes := downBytes + upBytes
	if totalBytes > 0 && r.cfg.ApihubURL != "" {
		r.reportTraffic(userID, edgeID, downBytes, upBytes)
		// 更新本地余额缓存
		if userID > 0 {
			kb := totalBytes / 1024
			if kb > 0 {
				r.balCacheMu.Lock()
				if e, ok := r.balCache[userID]; ok {
					e.balance -= kb
				}
				r.balCacheMu.Unlock()
			}
		}
	}
}

// ---------- 余额检查 ----------

func (r *relay) checkBalance(userID uint) bool {
	now := time.Now()
	r.balCacheMu.RLock()
	if e, ok := r.balCache[userID]; ok && now.Before(e.expireAt) {
		bal := e.balance
		r.balCacheMu.RUnlock()
		return bal > 0
	}
	r.balCacheMu.RUnlock()

	bal, err := r.fetchBalance(userID)
	if err != nil {
		return true
	}
	r.balCacheMu.Lock()
	r.balCache[userID] = &balanceEntry{balance: bal, expireAt: now.Add(balanceCacheTTL)}
	r.balCacheMu.Unlock()
	return bal > 0
}

func (r *relay) fetchBalance(userID uint) (int64, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/user/balance?user_id=%d", r.cfg.ApihubURL, userID))
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var result struct {
		Balance int64 `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}
	return result.Balance, nil
}

// ---------- 流量上报（审计 + 立即结算） ----------

func (r *relay) reportTraffic(userID uint, edgeID string, bytesIn, bytesOut int64) {
	body, _ := json.Marshal(map[string]interface{}{
		"user_id":   userID,
		"edge_id":   edgeID,
		"bytes_in":  bytesIn,
		"bytes_out": bytesOut,
	})
	resp, err := http.Post(r.cfg.ApihubURL+"/api/traffic/report", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("bridge: traffic report: %v", err)
		return
	}
	resp.Body.Close()
}
