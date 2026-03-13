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

// countryUnknown 无法解析时的默认国家代码，保证 nodes.country 一定有值
const countryUnknown = "unknown"

// countryFromAddr 根据连接对端 IP 得到国家：127.0.0.1/::1/localhost 固定 cn；否则用 ip2region 解析；解析不到返回 unknown
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

// relay 隧道转发：Edge 与 Client 连接管理、双向转发（TCP，QUIC 后续在 Client/Edge 侧加）
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
		// 不关闭 session，保留给 handleClientStream 转发用；Edge 断开时由 Open() 失败或后续清理感知
		return
	}
	if strings.HasPrefix(line, "CLIENT ") {
		clientToken := strings.TrimSpace(strings.TrimPrefix(line, "CLIENT "))
		stream.Close()
		country := r.countryFromAddr(remoteAddr)
		r.reportClientCountry(clientToken, country)
		defer session.Close()
		r.handleClientSession(session)
		return
	}
	stream.Close()
	session.Close()
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
	// 余额检查：余额<=0 拒绝转发
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
	// 仅转发 bufio 已缓冲的数据（如 "CONNECT host:port\n"），不能用 io.Copy(edgeStream, br) 否则会阻塞到 EOF
	if n := br.Buffered(); n > 0 {
		buf := make([]byte, n)
		nr, _ := br.Read(buf)
		if nr > 0 {
			if _, err := edgeStream.Write(buf[:nr]); err != nil {
				return
			}
		}
	}
	// in = 从 client Read = 用户上行(out)；out = 向 client Write = 用户下行(in)
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
	if userID > 0 && r.cfg.ApihubURL != "" {
		r.reportTraffic(userID, edgeID, downBytes, upBytes)
	}
}

func (r *relay) checkBalance(userID uint) bool {
	now := time.Now()
	r.balCacheMu.RLock()
	if e, ok := r.balCache[userID]; ok && now.Before(e.expireAt) {
		bal := e.balance
		r.balCacheMu.RUnlock()
		return bal > 0
	}
	r.balCacheMu.RUnlock()

	// 缓存未命中或已过期，从 apiHub 拉取
	bal, err := r.fetchBalance(userID)
	if err != nil {
		return true // apiHub 不可达时放行
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

	// 本地缓存同步扣减，避免下次连接还要查 apiHub
	totalKB := (bytesIn + bytesOut) / 1024
	if totalKB > 0 {
		r.balCacheMu.Lock()
		if e, ok := r.balCache[userID]; ok {
			e.balance -= totalKB
		}
		r.balCacheMu.Unlock()
	}
}
