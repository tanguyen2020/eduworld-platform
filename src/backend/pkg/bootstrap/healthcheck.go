package bootstrap

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// MaybeHealthcheck implements `<binary> healthcheck` for container HEALTHCHECK (distroless images
// have no shell or curl). It calls /readyz on the local admin port and exits 0 or 1.
// It returns without exiting when the first argument is not "healthcheck".
func MaybeHealthcheck(defaultAdminAddr string) {
	if len(os.Args) < 2 || os.Args[1] != "healthcheck" {
		return
	}
	addr := strings.TrimSpace(os.Getenv("EW_ADMIN_ADDR"))
	if addr == "" {
		addr = defaultAdminAddr
	}
	if err := probe(addr); err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

func probe(adminAddr string) error {
	host, port, err := net.SplitHostPort(adminAddr)
	if err != nil {
		return fmt.Errorf("parse admin addr %q: %w", adminAddr, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+"/readyz", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("readyz status %d", resp.StatusCode)
	}
	return nil
}
