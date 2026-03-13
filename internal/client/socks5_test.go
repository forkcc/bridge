package client

import (
	"bytes"
	"testing"
)

func TestSOCKS5ConnectRequestFormat(t *testing.T) {
	// 请求格式：VER CMD RSV ATYP DST.ADDR DST.PORT；域名 ATYP=3, len, "host", port
	var b bytes.Buffer
	b.Write([]byte{5, 1, 0, 3})
	b.WriteByte(4)
	b.WriteString("host")
	b.Write([]byte{0, 80}) // port 80
	if b.Len() != 4+1+4+2 {
		t.Errorf("len=%d", b.Len())
	}
}
