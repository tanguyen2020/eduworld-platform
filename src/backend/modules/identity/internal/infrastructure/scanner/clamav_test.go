package scanner

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strings"
	"testing"
)

// fakeClamd is a TEST DOUBLE (R17) of clamd INSTREAM: infected when the stream contains "EICAR".
func fakeClamd(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				cmd := make([]byte, 10)
				if _, err := io.ReadFull(c, cmd); err != nil || string(cmd) != "zINSTREAM\x00" {
					return
				}
				var data []byte
				size := make([]byte, 4)
				for {
					if _, err := io.ReadFull(c, size); err != nil {
						return
					}
					n := binary.BigEndian.Uint32(size)
					if n == 0 {
						break
					}
					b := make([]byte, n)
					if _, err := io.ReadFull(c, b); err != nil {
						return
					}
					data = append(data, b...)
				}
				if strings.Contains(string(data), "EICAR") {
					_, _ = c.Write([]byte("stream: Eicar-Test-Signature FOUND\x00"))
					return
				}
				_, _ = c.Write([]byte("stream: OK\x00"))
			}(c)
		}
	}()
	return ln.Addr().String()
}

func TestScan(t *testing.T) {
	s := ClamAV{Addr: fakeClamd(t)}
	clean, sig, err := s.Scan(context.Background(), strings.NewReader(strings.Repeat("student_external_id,name\n", 10000)))
	if err != nil || !clean || sig != "" {
		t.Fatalf("clean file = %v %q %v", clean, sig, err)
	}
	clean, sig, err = s.Scan(context.Background(), strings.NewReader("X5O!P%@AP EICAR-STANDARD-ANTIVIRUS-TEST-FILE"))
	if err != nil || clean || sig != "Eicar-Test-Signature" {
		t.Fatalf("infected = %v %q %v", clean, sig, err)
	}
	if _, _, err := (ClamAV{Addr: "127.0.0.1:1"}).Scan(context.Background(), strings.NewReader("x")); err == nil {
		t.Fatal("unreachable clamd must fail")
	}
}
