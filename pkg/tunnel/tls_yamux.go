package tunnel

import (
	"crypto/tls"
	"net"

	"github.com/hashicorp/yamux"
)

// Server 基于 TLS + yamux 的服务端会话
type Server struct {
	config *tls.Config
	listener net.Listener
}

// NewServer 创建隧道服务端，listener 为 TLS 监听器
func NewServer(listener net.Listener, config *tls.Config) *Server {
	return &Server{config: config, listener: listener}
}

// Accept 接受一个 TLS 连接并返回 yamux.Session
func (s *Server) Accept() (*yamux.Session, error) {
	conn, err := s.listener.Accept()
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Server(conn, s.config)
	if err := tlsConn.Handshake(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	session, err := yamux.Server(tlsConn, nil)
	if err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	return session, nil
}

// Close 关闭监听器
func (s *Server) Close() error {
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

// Dial 拨号到隧道服务端，返回 yamux.Session（用于 Client 端）
func Dial(addr string, tlsConfig *tls.Config) (*yamux.Session, error) {
	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return nil, err
	}
	session, err := yamux.Client(conn, nil)
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return session, nil
}

// Listen 在地址上创建 TLS 监听器
func Listen(addr string, config *tls.Config) (net.Listener, error) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	return tls.NewListener(ln, config), nil
}
