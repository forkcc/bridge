package edge

import (
	"bufio"
	"strings"
	"testing"
)

func TestParseConnectLine(t *testing.T) {
	line := "CONNECT example.com:443\n"
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "CONNECT ") {
		t.Fatal("expected CONNECT prefix")
	}
	target := strings.TrimSpace(strings.TrimPrefix(line, "CONNECT "))
	if target != "example.com:443" {
		t.Errorf("got %q", target)
	}
}

func TestParseConnectLineWithReader(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("CONNECT host:80\n"))
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "CONNECT ") {
		t.Fatal("expected CONNECT prefix")
	}
	target := strings.TrimSpace(strings.TrimPrefix(line, "CONNECT "))
	if target != "host:80" {
		t.Errorf("got %q", target)
	}
}
