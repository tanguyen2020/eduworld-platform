// Package scanner is the clamd adapter (INSTREAM over TCP, the clamav service of deploy/compose) used to
// scan SIS import files before they are read.
package scanner

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/identity/internal/application/port"
)

// ClamAV implements port.MalwareScanner.
type ClamAV struct {
	Addr    string // host:3310 (EW_CLAMAV_ADDR)
	Timeout time.Duration
}

var _ port.MalwareScanner = ClamAV{}

const chunk = 64 << 10

// Scan streams r to clamd; "stream: OK" is clean, "stream: <sig> FOUND" is infected.
func (c ClamAV) Scan(ctx context.Context, r io.Reader) (bool, string, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return false, "", fmt.Errorf("clamav: %w", err)
	}
	defer func() { _ = conn.Close() }()
	deadline := time.Now().Add(timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return false, "", fmt.Errorf("clamav: %w", err)
	}
	buf := make([]byte, chunk)
	size := make([]byte, 4)
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			binary.BigEndian.PutUint32(size, uint32(n)) //nolint:gosec // n <= chunk
			if _, err := conn.Write(size); err != nil {
				return false, "", fmt.Errorf("clamav: %w", err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return false, "", fmt.Errorf("clamav: %w", err)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return false, "", rerr
		}
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return false, "", fmt.Errorf("clamav: %w", err)
	}
	reply, err := bufio.NewReader(conn).ReadString(0)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, "", fmt.Errorf("clamav: %w", err)
	}
	reply = strings.TrimSpace(strings.TrimRight(reply, "\x00"))
	switch {
	case strings.HasSuffix(reply, " OK"):
		return true, "", nil
	case strings.HasSuffix(reply, " FOUND"):
		sig := strings.TrimSuffix(strings.TrimPrefix(reply, "stream: "), " FOUND")
		return false, sig, nil
	}
	return false, "", fmt.Errorf("clamav: unexpected reply %q", reply)
}
