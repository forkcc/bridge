package tunnel

import (
	"net"

	"github.com/hashicorp/yamux"
)

// Server 基于纯 TCP + yamux 的服务端（不加密、不压缩）
type Server struct {
	listener net.Listener
}

// NewServer 创建隧道服务端，listener 为 TCP 监听器
func NewServer(listener net.Listener) *Server {
	return &Server{listener: listener}
}

// Accept 接受一个 TCP 连接并返回 yamux.Session
func (s *Server) Accept() (*yamux.Session, error) {
	conn, err := s.listener.Accept()
	if err != nil {
		return nil, err
	}
	session, err := yamux.Server(conn, nil)
	if err != nil {
		_ = conn.Close()
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

// Dial 拨号到隧道服务端，返回 yamux.Session（纯 TCP，不加密）
func Dial(addr string) (*yamux.Session, error) {
	conn, err := net.Dial("tcp", addr)
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

// Listen 在地址上创建 TCP 监听器（不加密、不压缩）
func Listen(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}
