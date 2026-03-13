package client

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"strconv"
)

// serveSOCKS5 处理单条 SOCKS5 连接：握手、CONNECT、转发（c26 直连目标，c27 改走隧道）
func (s *Server) serveSOCKS5(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 256)
	if _, err := io.ReadAtLeast(conn, buf[:2], 2); err != nil {
		return
	}
	if buf[0] != 5 {
		return
	}
	nmethods := int(buf[1])
	if nmethods > 0 {
		if _, err := io.ReadFull(conn, buf[:nmethods]); err != nil {
			return
		}
	}
	if _, err := conn.Write([]byte{5, 0}); err != nil {
		return
	}
	if _, err := io.ReadAtLeast(conn, buf[:4], 4); err != nil {
		return
	}
	if buf[0] != 5 || buf[1] != 1 {
		conn.Write([]byte{5, 7, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	atyp := buf[3]
	var host string
	switch atyp {
	case 1:
		if _, err := io.ReadFull(conn, buf[:4]); err != nil {
			return
		}
		host = net.IP(buf[:4]).String()
	case 3:
		if _, err := io.ReadFull(conn, buf[:1]); err != nil {
			return
		}
		l := int(buf[0])
		if _, err := io.ReadFull(conn, buf[:l]); err != nil {
			return
		}
		host = string(buf[:l])
	case 4:
		if _, err := io.ReadFull(conn, buf[:16]); err != nil {
			return
		}
		host = net.IP(buf[:16]).String()
	default:
		conn.Write([]byte{5, 8, 0, 1, 0, 0, 0, 0, 0, 0})
		return
	}
	if _, err := io.ReadFull(conn, buf[:2]); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(buf[:2])
	targetAddr := net.JoinHostPort(host, strconv.Itoa(int(port)))
	var target net.Conn
	target, err := s.forwardViaTunnel(targetAddr)
	if err != nil {
		target, err = net.Dial("tcp", targetAddr)
		if err != nil {
			log.Printf("client: dial %s: %v", targetAddr, err)
			conn.Write([]byte{5, 5, 0, 1, 0, 0, 0, 0, 0, 0})
			return
		}
	}
	defer target.Close()
	if _, err := conn.Write([]byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	go io.Copy(target, conn)
	io.Copy(conn, target)
}
