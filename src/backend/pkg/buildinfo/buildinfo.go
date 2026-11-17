// Package buildinfo exposes build metadata injected with -ldflags at build time (F001 T007).
//
//	go build -ldflags "-X github.com/tanguyen2020/eduworld-platform/src/backend/pkg/buildinfo.Version=backend/v0.1.0 ..."
//
// Binary is the binary name (core-api, realtime, worker, migrate); Profile is only set for worker.
package buildinfo

// Values are overridden by -ldflags; defaults identify a local, unversioned build.
var (
	Binary    = "unknown"
	Profile   = ""
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)

// Info is the JSON shape served by GET /version.
type Info struct {
	Binary    string `json:"binary"`
	Profile   string `json:"profile,omitempty"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"build_time"`
}

// Get returns the build metadata. profile overrides the link-time Profile when the worker
// profile is chosen at runtime (--profile / EW_WORKER_PROFILE).
func Get(binary, profile string) Info {
	b := Binary
	if binary != "" {
		b = binary
	}
	p := Profile
	if profile != "" {
		p = profile
	}
	return Info{Binary: b, Profile: p, Version: Version, Commit: Commit, BuildTime: BuildTime}
}
