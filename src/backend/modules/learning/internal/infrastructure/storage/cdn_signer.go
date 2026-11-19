package storage

import (
	"crypto/md5" //nolint:gosec // nginx secure_link_md5 format (deploy/cdn/nginx.conf), not a password hash
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
)

// CdnSigner builds CDN URLs of media objects (F027-AC-06, ADR-0044 §2): <base>/<bucket>/<key>; objects of
// ew-media-private get the nginx secure_link parameters md5 = base64url(md5("<expires><uri> <secret>")) and
// expires, valid TTL (media_signed_url_ttl), so an expired URL is refused by the CDN.
type CdnSigner struct {
	BaseURL string        // EW_CDN_BASE_URL, e.g. http://localhost:8090
	Key     string        // EW_CDN_SIGNING_KEY (never logged)
	TTL     time.Duration // media_signed_url_ttl
}

var _ port.URLSigner = CdnSigner{}

const privateBucket = "ew-media-private"

// URL implements port.URLSigner.
func (s CdnSigner) URL(bucket, key string, now time.Time) (string, *time.Time) {
	uri := "/" + bucket + "/" + strings.TrimLeft(key, "/")
	base := strings.TrimRight(s.BaseURL, "/")
	if bucket != privateBucket {
		return base + uri, nil
	}
	exp := now.Add(s.TTL).Truncate(time.Second).UTC()
	e := strconv.FormatInt(exp.Unix(), 10)
	return base + uri + "?md5=" + s.sign(uri, e) + "&expires=" + e, &exp
}

func (s CdnSigner) sign(uri, expires string) string {
	sum := md5.Sum([]byte(expires + uri + " " + s.Key)) //nolint:gosec // nginx secure_link_md5
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Verify checks a signed URL the way the CDN does (secure_link): the signature matches and now is before expires.
// It is the reference implementation of the nginx rule, used by the tests (an expired URL is refused).
func (s CdnSigner) Verify(uri, md5Param, expires string, now time.Time) bool {
	e, err := strconv.ParseInt(expires, 10, 64)
	if err != nil || s.sign(uri, expires) != md5Param {
		return false
	}
	return now.Unix() < e
}
