package tunnel

import (
	"bytes"
	"testing"
)

func TestReadFrame(t *testing.T) {
	payload := []byte("hello")
	var buf bytes.Buffer
	if err := WriteFrame(&buf, payload); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFrame(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("got %q want %q", got, payload)
	}
}

func TestWriteFrame_TooLarge(t *testing.T) {
	payload := make([]byte, maxFrameSize+1)
	if err := WriteFrame(&bytes.Buffer{}, payload); err != ErrFrameTooLarge {
		t.Errorf("want ErrFrameTooLarge got %v", err)
	}
}
