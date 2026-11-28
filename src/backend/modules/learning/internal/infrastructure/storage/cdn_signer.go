package storage

import (
	"crypto/md5" //nolint:gosec // nginx secure_link_md5 format (deploy/cdn/nginx.conf), not a password hash
	"encoding/base64"
	"regexp"
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

// hlsPlaylist is the key of the playlist of an HLS variant (pkg/media VideoJobs: media/<sha256>/hls_<height>/index.m3u8);
// its segments (seg_%03d.ts) are relative URIs of the same directory.
var hlsPlaylist = regexp.MustCompile(`^media/[0-9a-f]{64}/hls_[0-9]+/[A-Za-z0-9._-]+\.m3u8$`)

// tokenPath is a path of a directory token: /ew-media-private/t/<md5>/<expires>/media/<sha256>/hls_<n>/<file>
// (deploy/cdn/nginx.conf, same regex).
var tokenPath = regexp.MustCompile(`^/ew-media-private/t/([A-Za-z0-9_-]{22})/([0-9]{1,12})(/media/[0-9a-f]{64}/hls_[0-9]+/)([A-Za-z0-9_-][A-Za-z0-9._-]*)$`)

// URL implements port.URLSigner.
func (s CdnSigner) URL(bucket, key string, now time.Time) (string, *time.Time) {
	uri := "/" + bucket + "/" + strings.TrimLeft(key, "/")
	base := strings.TrimRight(s.BaseURL, "/")
	if bucket != privateBucket {
		return base + uri, nil
	}
	exp := now.Add(s.TTL).Truncate(time.Second).UTC()
	e := strconv.FormatInt(exp.Unix(), 10)
	k := strings.TrimLeft(key, "/")
	if hlsPlaylist.MatchString(k) {
		// HLS (ADR-0044 §2, owner "chốt hết" 2026-11-28): the token signs the variant directory and sits in the path, so
		// the relative segment URIs of the playlist resolve to URLs signed with the same expiry; the bucket stays private.
		dir := k[:strings.LastIndex(k, "/")+1]
		return base + "/" + bucket + "/t/" + s.sign("/"+bucket+"/"+dir, e) + "/" + e + "/" + k, &exp
	}
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

// VerifyPath checks a directory-token path the way the CDN does (secure_link of the token location of
// deploy/cdn/nginx.conf): the token signs "/ew-media-private" + the variant directory of the path and now is before
// expires. Reference implementation of the nginx rule for the tests.
func (s CdnSigner) VerifyPath(path string, now time.Time) bool {
	m := tokenPath.FindStringSubmatch(path)
	if m == nil {
		return false
	}
	return s.Verify("/"+privateBucket+m[3], m[1], m[2], now)
}
