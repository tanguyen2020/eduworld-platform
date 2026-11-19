//go:build integration

// Package integration holds the integration tests of module game (F014): PostgreSQL with the module migrations
// (testcontainers), NATS JetStream for consumers and Valkey for the realtime adapter; the HTTP tests go through the
// real pkg/httpx chain with a fake OIDC issuer (httpxtest).
package integration
