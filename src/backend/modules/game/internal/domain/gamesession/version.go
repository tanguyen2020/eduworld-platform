package gamesession

import (
	"errors"
	"strconv"
	"strings"
)

// Version is a client version major.minor.patch with an optional pre-release (semver; same rule as the game
// login of module identity, min_client_version).
type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

// ErrVersion: the value is not major.minor.patch[-pre|+build].
var ErrVersion = errors.New("gamesession: invalid client version")

// ParseVersion parses a client version; build metadata (+...) is ignored, a pre-release ranks below the release.
func ParseVersion(s string) (Version, error) {
	core, pre := s, ""
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		core = s[:i]
		if s[i] == '-' {
			pre = s[i+1:]
			if j := strings.IndexByte(pre, '+'); j >= 0 {
				pre = pre[:j]
			}
		}
	}
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return Version{}, ErrVersion
	}
	var n [3]int
	for i, p := range parts {
		v, err := strconv.Atoi(p)
		if err != nil || v < 0 || p == "" {
			return Version{}, ErrVersion
		}
		n[i] = v
	}
	return Version{Major: n[0], Minor: n[1], Patch: n[2], Pre: pre}, nil
}

// Less reports v < o.
func (v Version) Less(o Version) bool {
	a, b := [3]int{v.Major, v.Minor, v.Patch}, [3]int{o.Major, o.Minor, o.Patch}
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	switch {
	case v.Pre == o.Pre, v.Pre == "":
		return false
	case o.Pre == "":
		return true
	}
	return v.Pre < o.Pre
}

// Supported reports whether client meets minVersion (min_client_version). An empty minimum accepts every
// version; an unparsable client version is unsupported; a broken minimum (reported at startup) accepts all.
func Supported(client, minVersion string) bool {
	if minVersion == "" {
		return true
	}
	m, err := ParseVersion(minVersion)
	if err != nil {
		return true
	}
	c, err := ParseVersion(client)
	if err != nil {
		return false
	}
	return !c.Less(m)
}
