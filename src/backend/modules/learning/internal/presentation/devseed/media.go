package devseed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// MediaSeed is a media file of a lesson file: either a small text file of the repository (File, e.g. a WebVTT
// subtitle) or a SYNTHETIC clip generated on the machine of the developer (Generate: ffmpeg test pattern and tone in a
// throwaway container, never committed — LOCAL_DEVELOPMENT §5).
type MediaSeed struct {
	Key      string         `json:"key"`
	Kind     string         `json:"kind"`
	Filename string         `json:"filename"`
	MIME     string         `json:"mime_type"`
	License  string         `json:"license"`
	File     string         `json:"file,omitempty"`
	Generate *MediaGenerate `json:"generate,omitempty"`
	// Variants are the variants the pipeline must have produced (F027, e.g. ogv_480/ogv_720 for the game client).
	Variants []string `json:"variants,omitempty"`
}

// MediaGenerate describes a synthetic clip: a video test pattern with a sine tone (Width×Height at FPS) or, without
// Width, a sine tone only. Only numbers: the generator builds the ffmpeg arguments itself.
type MediaGenerate struct {
	DurationS int `json:"duration_s"`
	Width     int `json:"width,omitempty"`
	Height    int `json:"height,omitempty"`
	FPS       int `json:"fps,omitempty"`
	ToneHz    int `json:"tone_hz"`
}

var mediaKinds = map[string][]string{"video": {"video/mp4"}, "audio": {"audio/ogg"}, "subtitle": {"text/vtt"}}

// Validate checks a media seed (kinds and types the seed can produce, a devseed- file name).
func (m MediaSeed) Validate() error {
	types, ok := mediaKinds[m.Kind]
	switch {
	case !ok || !slices.Contains(types, m.MIME):
		return fmt.Errorf("devseed: media %q: kind/mime_type must be one of %v", m.Key, mediaKinds)
	case !strings.HasPrefix(m.Filename, "devseed-") || len(m.Filename) > 255 || strings.ContainsAny(m.Filename, `/\`):
		return fmt.Errorf("devseed: media %q: filename must start with devseed- (no path)", m.Key)
	case m.License == "" || len(m.License) > 255:
		return fmt.Errorf("devseed: media %q: license required", m.Key)
	case (m.File == "") == (m.Generate == nil):
		return fmt.Errorf("devseed: media %q: exactly one of file and generate", m.Key)
	case m.File != "" && m.Kind != "subtitle":
		return fmt.Errorf("devseed: media %q: only subtitles come from a file (no binary in the repository)", m.Key)
	}
	if g := m.Generate; g != nil {
		video := m.Kind == "video"
		switch {
		case g.DurationS < 1 || g.DurationS > 600 || g.ToneHz < 20 || g.ToneHz > 8000:
			return fmt.Errorf("devseed: media %q: duration_s 1..600 and tone_hz 20..8000", m.Key)
		case video && (g.Width < 16 || g.Width > 1920 || g.Height < 16 || g.Height > 1080 || g.FPS < 1 || g.FPS > 60 || g.Width%2+g.Height%2 != 0):
			return fmt.Errorf("devseed: media %q: even width/height ≤ 1920×1080 and fps 1..60 for a video", m.Key)
		case !video && (g.Width != 0 || g.Height != 0 || g.FPS != 0):
			return fmt.Errorf("devseed: media %q: width/height/fps only for a video", m.Key)
		case m.Kind == "subtitle":
			return fmt.Errorf("devseed: media %q: a subtitle cannot be generated", m.Key)
		}
	}
	return nil
}

// Generator produces the bytes of a synthetic clip (cmd/seed-media: ffmpeg in a throwaway container).
type Generator interface {
	Generate(ctx context.Context, kind string, g MediaGenerate) ([]byte, error)
}

// MediaClient uploads media through the media API of the CMS (presignMediaUpload → PUT to the presigned URL →
// completeMediaUpload) as a platform user with media.upload (System Admin) and waits for the worker (F027) to make it
// ready.
type MediaClient struct {
	BaseURL string // public gateway, ends with /api/v1
	HTTP    *http.Client
	Token   string // platform user with media.upload (platform media)
	// Relogin returns a fresh token when the API answers 401 (portal access tokens are short-lived and the worker may
	// take minutes); nil = no retry.
	Relogin func(ctx context.Context) (string, error)
	// RepoDir resolves MediaSeed.File (src/backend for `go -C src/backend run`).
	RepoDir   string
	Generator Generator
	// S3Addr, when set, is the host:port the presigned PUT is sent to while the signed Host header is kept: the URL
	// names the object storage as the containers see it (http://floci:4566), the seed runs on the host (localhost:4566).
	S3Addr string
	Wait   time.Duration // how long to wait for ready (default 10 min)
	// TenantID, when set, makes the media tenant media of that tenant (X-Tenant-Id; System Admin writes name their
	// target tenant): platform media cannot be assigned to tenants yet, so a tenant student only sees tenant media.
	TenantID string
	Poll     time.Duration // poll interval (default 3 s)
}

// Ensure returns the ready asset of m (found again by file name, uploaded when missing) and true when it uploaded.
func (c *MediaClient) Ensure(ctx context.Context, m MediaSeed) (uuid.UUID, bool, error) {
	a, err := c.find(ctx, m)
	if err != nil {
		return uuid.Nil, false, err
	}
	uploaded := false
	if a == nil || str(a, "status") == "quarantined" {
		if a, err = c.upload(ctx, m, a); err != nil {
			return uuid.Nil, false, err
		}
		uploaded = true
	}
	id, err := uuid.Parse(str(a, "asset_id"))
	if err != nil {
		return uuid.Nil, uploaded, fmt.Errorf("media without asset_id: %w", err)
	}
	ready, err := c.ready(ctx, id)
	if err != nil {
		return id, uploaded, err
	}
	have := map[string]bool{}
	if vs, ok := ready["variants"].([]any); ok {
		for _, v := range vs {
			if vm, ok := v.(map[string]any); ok {
				have[str(vm, "variant")] = true
			}
		}
	}
	for _, v := range m.Variants {
		if !have[v] {
			return id, uploaded, fmt.Errorf("media %s is ready without variant %s", m.Filename, v)
		}
	}
	return id, uploaded, nil
}

func (c *MediaClient) owner() string {
	if c.TenantID != "" {
		return "tenant"
	}
	return "platform"
}

// call sends one request with the token of the client (and X-Tenant-Id of tenant media), signing in again once on 401.
func (c *MediaClient) call(ctx context.Context, method, path string, h map[string]string, body, out any) (string, error) {
	if c.TenantID != "" {
		h = map[string]string{"X-Tenant-Id": c.TenantID}
	}
	etag, err := Do(ctx, c.HTTP, c.BaseURL, c.Token, method, path, h, body, out)
	if c.Relogin == nil || !IsStatus(err, http.StatusUnauthorized) {
		return etag, err
	}
	t, lerr := c.Relogin(ctx)
	if lerr != nil {
		return "", lerr
	}
	c.Token = t
	return Do(ctx, c.HTTP, c.BaseURL, c.Token, method, path, h, body, out)
}

// find returns the platform asset named m.Filename (listMediaAssets, every page), nil when there is none.
func (c *MediaClient) find(ctx context.Context, m MediaSeed) (map[string]any, error) {
	cursor := ""
	for range 50 {
		q := url.Values{"kind": {m.Kind}, "q": {m.Filename}, "limit": {"100"}}
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var p struct {
			Data []map[string]any `json:"data"`
			Page struct {
				NextCursor *string `json:"next_cursor"`
				HasMore    bool    `json:"has_more"`
			} `json:"page"`
		}
		if _, err := c.call(ctx, http.MethodGet, "/cms/media?"+q.Encode(), nil, nil, &p); err != nil {
			return nil, err
		}
		for _, a := range p.Data {
			if str(a, "original_filename") == m.Filename && ownedBy(a, c.owner()) && str(a, "status") != "rejected" {
				return a, nil
			}
		}
		if !p.Page.HasMore || p.Page.NextCursor == nil || *p.Page.NextCursor == "" {
			return nil, nil
		}
		cursor = *p.Page.NextCursor
	}
	return nil, fmt.Errorf("media %s: too many pages", m.Filename)
}

// content returns the bytes of m (repository file or generated clip).
func (c *MediaClient) content(ctx context.Context, m MediaSeed) ([]byte, error) {
	if m.File != "" {
		return os.ReadFile(filepath.Join(c.RepoDir, filepath.FromSlash(m.File))) //nolint:gosec // seed file of the repository
	}
	if c.Generator == nil {
		return nil, errors.New("no media generator configured")
	}
	return c.Generator.Generate(ctx, m.Kind, *m.Generate)
}

// upload presigns, PUTs and completes the upload of m; a quarantined asset of an interrupted run is completed first
// (its object may already be there), then uploaded again when that fails.
func (c *MediaClient) upload(ctx context.Context, m MediaSeed, quarantined map[string]any) (map[string]any, error) {
	if quarantined != nil {
		var a map[string]any
		if _, err := c.call(ctx, http.MethodPost, "/cms/media/"+str(quarantined, "asset_id")+":complete-upload", nil, nil, &a); err == nil &&
			str(a, "status") != "quarantined" && str(a, "status") != "rejected" {
			return a, nil
		}
	}
	data, err := c.content(ctx, m)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(data)
	body := map[string]any{"kind": m.Kind, "filename": m.Filename, "mime_type": m.MIME, "size_bytes": len(data),
		"sha256": hex.EncodeToString(sum[:]), "license": m.License, "owner_scope": c.owner()}
	if c.TenantID != "" {
		body["tenant_id"] = c.TenantID
	}
	var pre struct {
		AssetID     string            `json:"asset_id"`
		Status      string            `json:"status"`
		UploadURL   string            `json:"upload_url"`
		Headers     map[string]string `json:"upload_headers"`
		DuplicateOf string            `json:"duplicate_of"`
	}
	if _, err := c.call(ctx, http.MethodPost, "/cms/media:presign-upload", nil, body, &pre); err != nil {
		return nil, err
	}
	if pre.DuplicateOf != "" || pre.UploadURL == "" {
		// The same bytes are already an asset of the platform (F027-AC-01): reuse it.
		return map[string]any{"asset_id": pre.AssetID, "status": pre.Status}, nil
	}
	if err := c.put(ctx, pre.UploadURL, pre.Headers, data); err != nil {
		return nil, err
	}
	var a map[string]any
	if _, err := c.call(ctx, http.MethodPost, "/cms/media/"+pre.AssetID+":complete-upload", nil, nil, &a); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "media uploaded", "asset_id", pre.AssetID, "filename", m.Filename, "bytes", len(data), "status", str(a, "status"))
	return a, nil
}

// put sends the object to the presigned URL (no bearer token: the URL is the credential).
func (c *MediaClient) put(ctx context.Context, rawURL string, headers map[string]string, data []byte) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("presigned url: %w", err)
	}
	host := u.Host
	if c.S3Addr != "" {
		u.Host = c.S3Addr
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Host = host
	req.ContentLength = int64(len(data))
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	res, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("PUT presigned object: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("PUT presigned object: %d %s", res.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// ready polls getMediaAsset until the worker made the asset ready (rejected → error).
func (c *MediaClient) ready(ctx context.Context, id uuid.UUID) (map[string]any, error) {
	wait, poll := c.Wait, c.Poll
	if wait <= 0 {
		wait = 10 * time.Minute
	}
	if poll <= 0 {
		poll = 3 * time.Second
	}
	deadline := time.Now().Add(wait)
	for {
		var a map[string]any
		if _, err := c.call(ctx, http.MethodGet, "/cms/media/"+id.String(), nil, nil, &a); err != nil {
			return nil, err
		}
		switch str(a, "status") {
		case "ready":
			return a, nil
		case "rejected":
			return nil, fmt.Errorf("media %s rejected: %s", id, str(a, "reject_reason"))
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("media %s still %s after %s: is the worker (profile events/all) running?", id, str(a, "status"), wait)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(poll):
		}
	}
}
