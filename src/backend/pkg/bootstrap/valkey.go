package bootstrap

import (
	"context"
	"sync"

	"github.com/valkey-io/valkey-go"
)

// ValkeyPinger connects lazily so a binary starts while Valkey is down and reports not ready
// instead of crashing (F001-AC-05). Module-scoped clients with ACL users come with F002/F003.
type ValkeyPinger struct {
	addr string

	mu     sync.Mutex
	client valkey.Client
}

// NewValkeyPinger returns a pinger for addr (host:port).
func NewValkeyPinger(addr string) *ValkeyPinger { return &ValkeyPinger{addr: addr} }

// Ping sends PING, creating the client on first use or after a failed creation.
func (p *ValkeyPinger) Ping(ctx context.Context) error {
	c, err := p.get()
	if err != nil {
		return err
	}
	return c.Do(ctx, c.B().Ping().Build()).Error()
}

func (p *ValkeyPinger) get() (valkey.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		return p.client, nil
	}
	c, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{p.addr}, DisableCache: true})
	if err != nil {
		return nil, err
	}
	p.client = c
	return c, nil
}

// Close releases the client.
func (p *ValkeyPinger) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.client != nil {
		p.client.Close()
		p.client = nil
	}
}
