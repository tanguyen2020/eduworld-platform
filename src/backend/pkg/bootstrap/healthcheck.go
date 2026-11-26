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

// MaybeHealthcheck implements `<binary> healthcheck [--endpoint=/readyz|/livez]` for container HEALTHCHECK
// (distroless images have no shell or curl). It calls the endpoint (default /readyz, used by the local compose
// depends_on) on the local admin port and exits 0 or 1. The Swarm healthcheck of staging/production passes
// --endpoint=/livez so a dependency outage never restarts tasks in cascade (DEPLOYMENT §14, F130-AC-06).
// It returns without exiting when the first argument is not "healthcheck".
func MaybeHealthcheck(defaultAdminAddr string) {
	if len(os.Args) < 2 || os.Args[1] != "healthcheck" {
		return
	}
	addr := strings.TrimSpace(os.Getenv("EW_ADMIN_ADDR"))
	if addr == "" {
		addr = defaultAdminAddr
	}
	endpoint, err := HealthcheckEndpoint(os.Args[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(2)
	}
	if err := probe(addr, endpoint); err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
	os.Exit(0)
}

// HealthcheckEndpoint parses the arguments after "healthcheck": only /readyz (default) and /livez are allowed.
func HealthcheckEndpoint(args []string) (string, error) {
	endpoint := "/readyz"
	for _, a := range args {
		v, ok := strings.CutPrefix(a, "--endpoint=")
		if !ok || (v != "/readyz" && v != "/livez") {
			return "", fmt.Errorf("unsupported argument %q (want --endpoint=/readyz or --endpoint=/livez)", a)
		}
		endpoint = v
	}
	return endpoint, nil
}

func probe(adminAddr, endpoint string) error {
	host, port, err := net.SplitHostPort(adminAddr)
	if err != nil {
		return fmt.Errorf("parse admin addr %q: %w", adminAddr, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+net.JoinHostPort(host, port)+endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s status %d", endpoint, resp.StatusCode)
	}
	return nil
}
