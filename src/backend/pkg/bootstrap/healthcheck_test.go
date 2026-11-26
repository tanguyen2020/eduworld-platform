package bootstrap

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

// F130-AC-06: the Swarm healthcheck runs `<binary> healthcheck --endpoint=/livez`; /readyz stays the default of the
// local compose; any other argument is refused (no arbitrary URL probing).
func TestHealthcheckEndpoint(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
		ok   bool
	}{
		{nil, "/readyz", true},
		{[]string{"--endpoint=/livez"}, "/livez", true},
		{[]string{"--endpoint=/readyz"}, "/readyz", true},
		{[]string{"--endpoint=/metrics"}, "", false},
		{[]string{"--endpoint=http://evil/"}, "", false},
		{[]string{"/livez"}, "", false},
	} {
		got, err := HealthcheckEndpoint(c.args)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("HealthcheckEndpoint(%v) = %q, %v", c.args, got, err)
		}
	}
}

func TestProbeUsesTheChosenEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/livez" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable) // a dependency is down: not ready, but alive
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	if err := probe(":"+port, "/livez"); err != nil {
		t.Fatalf("livez probe failed: %v", err)
	}
	if err := probe(":"+port, "/readyz"); err == nil {
		t.Fatal("readyz probe passed on 503")
	}
}
