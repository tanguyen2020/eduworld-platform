//go:build integration

package integration

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/valkey-io/valkey-go"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/command"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/query"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity/plugins"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/cache"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/mediaproc"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/persistence/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/qbschema"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/infrastructure/storage"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/authz"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/page"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/tenancy"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/testkit"
)

// mediaKit is the F027 part of the HTTP environment: floci (S3 with SigV4 enforced, the image of deploy/compose),
// Valkey for the preview sessions, and clamd replaced by a TEST DOUBLE (R17) that flags the EICAR string — the real
// ClamAV is exercised by the quickstart (compose profile media), not by these tests.
type mediaKit struct {
	s3      *storage.S3
	signer  storage.CdnSigner
	clamd   *fakeClamd
	cmd     command.Media
	math    command.MathAssets
	qry     query.MediaLibrary
	preview command.Preview
	store   cache.PreviewStore
	cache   cache.SnapshotCache // F026 snapshots and release pointers
}

// testCDNKey is the CDN signing key of the tests (TEST FIXTURE, not a secret of any environment).
const testCDNKey = "test-only-cdn-signing-key"

func newMediaKit(ctx context.Context, t *testing.T, e *env, codec *page.Codec, schema *qbschema.Validator) *mediaKit {
	t.Helper()
	endpoint := startFloci(ctx, t)
	s3, err := storage.New(storage.Config{Endpoint: endpoint, AccessKey: "test", SecretKey: "test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range []string{pkgmedia.BucketQuarantine, pkgmedia.BucketPublic, pkgmedia.BucketPrivate} {
		if err := s3.EnsureBucket(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	_, vaddr := testkit.StartValkey(ctx, t)
	vk, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{vaddr}, DisableCache: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(vk.Close)
	k := &mediaKit{s3: s3, signer: storage.CdnSigner{BaseURL: "http://cdn.test", Key: testCDNKey, TTL: command.MediaSignedURLTTL},
		clamd: startFakeClamd(t), store: cache.PreviewStore{C: vk}, cache: cache.SnapshotCache{C: vk}}
	proc := mediaproc.New(k.clamd.addr)
	k.cmd = command.Media{UoW: e.uow, Store: write.MediaRepo{}, Storage: s3, Signer: k.signer, Processor: proc, Events: write.Publisher{W: e.writer},
		Audit: write.Auditor{R: e.rec}, WorkDir: t.TempDir(), Metrics: observability.MediaCounter{}}
	k.math = command.MathAssets{Store: write.MathRepo{}, Storage: s3, Metrics: observability.MediaCounter{}}
	k.qry = query.MediaLibrary{Session: e.read, Model: read.MediaModel{}, Signer: k.signer, Cursors: codec}
	k.preview = command.Preview{UoW: e.uow, Session: e.read, Questions: read.QuestionModel{}, Registry: question.Full(), Schema: schema, Store: k.store,
		Docs: read.DocModel{}, Activities: plugins.Registry(), QuestionVersions: read.QuestionModel{}}
	return k
}

// startFloci starts floci with S3 authentication enforced (deploy/compose) and returns its endpoint.
func startFloci(ctx context.Context, t *testing.T) string {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{Image: "floci/floci:2.5.0", ExposedPorts: []string{"4566/tcp"},
			Env:        map[string]string{"FLOCI_SERVICES_S3_ENFORCE_AUTH": "true"},
			WaitingFor: wait.ForListeningPort("4566/tcp").WithStartupTimeout(2 * time.Minute)},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start floci: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	host, _ := c.Host(ctx)
	p, _ := c.MappedPort(ctx, "4566/tcp")
	endpoint := "http://" + host + ":" + p.Port()
	waitFor(t, time.Minute, "floci ready", func() bool {
		res, err := http.Get(endpoint + "/") //nolint:noctx // readiness probe
		if err != nil {
			return false
		}
		_ = res.Body.Close()
		return res.StatusCode < 500
	})
	return endpoint
}

// fakeClamd is a TEST DOUBLE (R17) of clamd INSTREAM: infected when the stream contains "EICAR"; down when
// unavailable is set (the connection is closed without answer).
type fakeClamd struct {
	addr        string
	unavailable atomic.Bool
	scans       atomic.Int32
}

func startFakeClamd(t *testing.T) *fakeClamd {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	f := &fakeClamd{addr: ln.Addr().String()}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				if f.unavailable.Load() {
					return
				}
				f.scans.Add(1)
				cmd := make([]byte, 10)
				if _, err := io.ReadFull(c, cmd); err != nil {
					return
				}
				var data []byte
				for {
					var n uint32
					if err := binary.Read(c, binary.BigEndian, &n); err != nil || n == 0 {
						break
					}
					b := make([]byte, n)
					if _, err := io.ReadFull(c, b); err != nil {
						return
					}
					data = append(data, b...)
				}
				reply := "stream: OK\x00"
				if bytes.Contains(data, []byte("EICAR")) {
					reply = "stream: Eicar-Test-Signature FOUND\x00"
				}
				_, _ = c.Write([]byte(reply))
			}(c)
		}
	}()
	return f
}

func sha(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// presign asks an upload URL (presignMediaUpload).
func (c *curEnv) presign(t *testing.T, sub, kind, mime string, data []byte, extra map[string]any) resp {
	t.Helper()
	body := map[string]any{"kind": kind, "filename": "file-" + kind, "mime_type": mime, "size_bytes": len(data), "sha256": sha(data),
		"license": "CC BY 4.0"}
	if kind == "image" {
		body["alt_text"] = "Hình minh họa"
	}
	for k, v := range extra {
		if v == nil {
			delete(body, k)
			continue
		}
		body[k] = v
	}
	return c.do(t, http.MethodPost, "/cms/media:presign-upload", sub, idem(), body)
}

// put uploads data with the presigned URL and headers of a presign answer.
func putUpload(t *testing.T, r resp, data []byte) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, r.str("upload_url"), bytes.NewReader(data)) //nolint:noctx // test upload
	if h, ok := r.body["upload_headers"].(map[string]any); ok {
		for k, v := range h {
			req.Header.Set(k, v.(string))
		}
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("presigned PUT: status %d", res.StatusCode)
	}
}

// upload runs presign → PUT → complete and returns the asset id and the completion answer.
func (c *curEnv) upload(t *testing.T, sub, kind, mime string, data []byte, extra map[string]any) (string, resp) {
	t.Helper()
	p := must(t, c.presign(t, sub, kind, mime, data, extra), http.StatusCreated, "presign "+kind)
	putUpload(t, p, data)
	id := p.str("asset_id")
	return id, c.do(t, http.MethodPost, "/cms/media/"+id+":complete-upload", sub, idem(), nil)
}

// process runs the media worker step of an asset the way learning.media_worker does: in the scope of the tenant
// of its media.uploaded event (platform scope for platform media).
func (c *curEnv) process(ctx context.Context, t *testing.T, id string) error {
	t.Helper()
	var tenant *uuid.UUID
	var raw []byte
	if err := c.relay.QueryRowContext(ctx, `SELECT tenant_id, envelope FROM infra.outbox_events WHERE event_type = 'media.uploaded'
		AND envelope->'payload'->>'asset_id' = $1`, id).Scan(&tenant, &raw); err != nil {
		t.Fatalf("media.uploaded of %s: %v", id, err)
	}
	var sc tenancy.Scope
	if tenant == nil {
		ps, err := authz.SystemPlatformScope("consumer-learning-media-worker")
		if err != nil {
			t.Fatal(err)
		}
		sc = ps
	} else {
		ts, err := authz.EventScope(*tenant)
		if err != nil {
			t.Fatal(err)
		}
		sc = ts
	}
	var env struct {
		Payload struct {
			AssetID uuid.UUID `json:"asset_id"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return c.uow.InTenantTx(tenancy.WithModule(ctx, "learning"), sc, func(ctx context.Context) error {
		return c.media.cmd.ProcessMedia(ctx, env.Payload.AssetID)
	})
}

// object reads an object of floci with its headers.
func (c *curEnv) object(ctx context.Context, t *testing.T, bucket, key string) []byte {
	t.Helper()
	rc, _, err := c.media.s3.Open(ctx, bucket, key)
	if err != nil {
		t.Fatalf("open %s/%s: %v", bucket, key, err)
	}
	defer func() { _ = rc.Close() }()
	b, _ := io.ReadAll(rc)
	return b
}

func (c *curEnv) mediaEvents(ctx context.Context, t *testing.T, typ, id string) int {
	t.Helper()
	return count(ctx, t, c.relay, `SELECT count(*) FROM infra.outbox_events WHERE event_type = $1 AND envelope->'payload'->>'asset_id' = $2`, typ, id)
}

func variantKey(r resp, name string) string {
	for _, v := range r.items("variants") {
		if v["variant"] == name {
			s, _ := v["key"].(string)
			return s
		}
	}
	return ""
}

func variantURL(r resp, name string) string {
	for _, v := range r.items("variants") {
		if v["variant"] == name {
			s, _ := v["url"].(string)
			return s
		}
	}
	return ""
}

var _ = strings.Contains
