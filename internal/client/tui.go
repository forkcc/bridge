package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GetEdgeStatus 返回当前隧道绑定的 edge 与是否已连接（供 TUI 显示）
func (s *Server) GetEdgeStatus() (edgeID string, connected bool) {
	ts := s.getTunnelState()
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if ts.session == nil || ts.edgeID == "" {
		return "", false
	}
	return ts.edgeID, true
}

// FetchBalance 拉取当前用户余额（供 TUI 显示）
func (s *Server) FetchBalance() (int64, error) {
	url := s.cfg.BridgeURL + "/api/user/balance?token=" + s.cfg.Token
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	var out struct {
		Balance int64 `json:"balance"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return 0, err
	}
	return out.Balance, nil
}

type tickMsg struct{ t time.Time }

type tuiModel struct {
	server     *Server
	edgeID     string
	connected  bool
	balance    int64
	balanceErr string
	width      int
	height     int
	// 网速统计
	prevDown   int64
	prevUp     int64
	prevTime   time.Time
	speedDown  float64 // bytes/s
	speedUp    float64 // bytes/s
}

func newTUIModel(s *Server) tuiModel {
	return tuiModel{
		server:   s,
		prevDown: atomic.LoadInt64(&totalBytesDown),
		prevUp:   atomic.LoadInt64(&totalBytesUp),
		prevTime: time.Now(),
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg{t} })
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		_, connected := m.server.GetEdgeStatus()
		if !connected {
			go func() { _, _, _ = m.server.ensureTunnel() }()
		}
		m.edgeID, m.connected = m.server.GetEdgeStatus()
		m.balanceErr = ""
		if b, err := m.server.FetchBalance(); err != nil {
			m.balanceErr = err.Error()
		} else {
			m.balance = b
		}
		now := time.Now()
		curDown := atomic.LoadInt64(&totalBytesDown)
		curUp := atomic.LoadInt64(&totalBytesUp)
		elapsed := now.Sub(m.prevTime).Seconds()
		if elapsed > 0 {
			m.speedDown = float64(curDown-m.prevDown) / elapsed
			m.speedUp = float64(curUp-m.prevUp) / elapsed
		}
		m.prevDown = curDown
		m.prevUp = curUp
		m.prevTime = now
		return m, tea.Tick(2*time.Second, func(t time.Time) tea.Msg { return tickMsg{t} })
	}
	return m, nil
}

func (m tuiModel) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	okStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	noStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))

	edgeStatus := "未连接"
	if m.connected && m.edgeID != "" {
		edgeStatus = okStyle.Render("已连接 ") + m.edgeID
	} else {
		edgeStatus = noStyle.Render("未连接")
	}

	balanceStr := fmt.Sprintf("%d", m.balance)
	if m.balanceErr != "" {
		balanceStr = noStyle.Render("获取失败")
	}

	speedStr := fmt.Sprintf("↓ %s  ↑ %s", formatSpeed(m.speedDown), formatSpeed(m.speedUp))

	s := titleStyle.Render("proxy-bridge client") + "\n\n"
	s += labelStyle.Render("Edge: ") + edgeStatus + "\n"
	s += labelStyle.Render("网速: ") + speedStr + "\n"
	s += labelStyle.Render("余额: ") + balanceStr + "\n\n"
	s += labelStyle.Render("每 2 秒刷新 · 按 q 或 Ctrl+C 退出（退出后 client 与 SOCKS5 一并结束）")
	return lipgloss.NewStyle().Padding(0, 1).Render(s)
}

// RunTUI 运行 TUI（阻塞直到退出）；调用方需在后台启动 SOCKS5 与心跳
func (s *Server) RunTUI() error {
	p := tea.NewProgram(newTUIModel(s), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
