package world

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"slices"
)

var (
	shaRe     = regexp.MustCompile(`^[0-9a-f]{64}$`)
	semverRe  = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
	platforms = []string{"windows", "android", "ios", "web", "macos", "linux"}
)

// Level is one version of a level package (payload of specs/schemas/level-package.schema.json): the zone bundle of an
// area.
type Level struct {
	SchemaVersion  int      `json:"schema_version"`
	LevelID        string   `json:"level_id"`
	AreaID         string   `json:"area_id"`
	BundleID       string   `json:"bundle_id"`
	BundleURL      string   `json:"bundle_url"`
	BundleSHA256   string   `json:"bundle_sha256"`
	SizeBytes      int64    `json:"size_bytes"`
	Platforms      []string `json:"platforms"`
	RequiresClient string   `json:"requires_client,omitempty"`
	Placeholder    bool     `json:"placeholder,omitempty"`
}

// ParseLevel decodes and checks a level package payload.
func ParseLevel(raw []byte) (Level, error) {
	var l Level
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&l); err != nil {
		return l, invalid("level package: %v", err)
	}
	u, err := url.Parse(l.BundleURL)
	switch {
	case l.SchemaVersion != 1:
		return l, invalid("level package: schema_version must be 1")
	case !uuidRe.MatchString(l.LevelID) || !ValidAreaID(l.AreaID) || !codeRe.MatchString(l.BundleID):
		return l, invalid("level package: level_id, area_id, bundle_id")
	case err != nil || u.Scheme != "https" || u.Host == "" || len(l.BundleURL) > 2048:
		return l, invalid("level package %s: bundle_url must be an https URL", l.LevelID)
	case !shaRe.MatchString(l.BundleSHA256) || l.SizeBytes < 0:
		return l, invalid("level package %s: bundle_sha256, size_bytes", l.LevelID)
	case len(l.Platforms) == 0 || len(l.Platforms) > 8:
		return l, invalid("level package %s: 1..8 platforms", l.LevelID)
	case l.RequiresClient != "" && !semverRe.MatchString(l.RequiresClient):
		return l, invalid("level package %s: requires_client", l.LevelID)
	}
	for i, p := range l.Platforms {
		if !slices.Contains(platforms, p) || slices.Contains(l.Platforms[:i], p) {
			return l, invalid("level package %s: platform %q", l.LevelID, p)
		}
	}
	return l, nil
}
