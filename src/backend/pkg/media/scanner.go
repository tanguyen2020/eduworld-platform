package media

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
)

// Verdict of a virus scan.
type Verdict struct {
	Clean     bool
	Signature string // the matched signature when infected (never logged above DEBUG with file names)
}

// Clamd scans streams with clamd INSTREAM over TCP (the clamav service of deploy/compose, profile media).
type Clamd struct {
	Addr    string // host:3310 (EW_CLAMAV_ADDR)
	Timeout time.Duration
}

const clamdChunk = 64 << 10

// Scan streams r to clamd: "stream: OK" is clean, "stream: <sig> FOUND" infected. Network failures are
// ErrScannerUnavailable (the worker retries and the asset stays processing, never ready unscanned).
func (c Clamd) Scan(ctx context.Context, r io.Reader) (Verdict, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	d := net.Dialer{Timeout: 5 * time.Second}
	conn, err := d.DialContext(ctx, "tcp", c.Addr)
	if err != nil {
		return Verdict{}, fmt.Errorf("%w: %w", ErrScannerUnavailable, err)
	}
	defer func() { _ = conn.Close() }()
	deadline := time.Now().Add(timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	_ = conn.SetDeadline(deadline)
	unavailable := func(err error) (Verdict, error) { return Verdict{}, fmt.Errorf("%w: %w", ErrScannerUnavailable, err) }
	if _, err := conn.Write([]byte("zINSTREAM\x00")); err != nil {
		return unavailable(err)
	}
	buf := make([]byte, clamdChunk)
	size := make([]byte, 4)
	for {
		n, rerr := r.Read(buf)
		if n > 0 {
			binary.BigEndian.PutUint32(size, uint32(n)) //nolint:gosec // n <= clamdChunk
			if _, err := conn.Write(size); err != nil {
				return unavailable(err)
			}
			if _, err := conn.Write(buf[:n]); err != nil {
				return unavailable(err)
			}
		}
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			return Verdict{}, rerr
		}
	}
	if _, err := conn.Write([]byte{0, 0, 0, 0}); err != nil {
		return unavailable(err)
	}
	reply, err := bufio.NewReader(conn).ReadString(0)
	if err != nil && !errors.Is(err, io.EOF) {
		return unavailable(err)
	}
	reply = strings.TrimSpace(strings.TrimRight(reply, "\x00"))
	switch {
	case strings.HasSuffix(reply, " OK"):
		return Verdict{Clean: true}, nil
	case strings.HasSuffix(reply, " FOUND"):
		return Verdict{Signature: strings.TrimSuffix(strings.TrimPrefix(reply, "stream: "), " FOUND")}, nil
	}
	return unavailable(fmt.Errorf("unexpected clamd reply %q", reply))
}
