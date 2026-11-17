// Package migrations embeds the Atlas migration directory of module learning so binaries can require a
// minimum revision at readiness time (they never apply migrations themselves — F001-AC-03).
package migrations

import (
	"embed"
	"io/fs"
	"sort"
	"strings"
)

//go:embed *.sql
var files embed.FS

// Latest returns the version prefix of the newest migration file (Atlas revision format).
func Latest() string {
	entries, err := fs.ReadDir(files, ".")
	if err != nil {
		return ""
	}
	versions := make([]string, 0, len(entries))
	for _, e := range entries {
		if v, _, ok := strings.Cut(e.Name(), "_"); ok {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return ""
	}
	sort.Strings(versions)
	return versions[len(versions)-1]
}
