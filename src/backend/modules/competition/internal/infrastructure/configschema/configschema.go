// Package configschema checks competition config payloads against the embedded copies of the seven JSON Schemas of
// specs/schemas (F108 T014, F108-AC-04): matchmaking-config, ranking-rules, season, tournament-template,
// challenge-template, leaderboard-definition and competition-reward. The copies are kept equal to the specs by
// TestEmbeddedCopiesMatchSpecs.
package configschema

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/competition/internal/domain/gameconfig"
)

// Files are the embedded copies of the schemas.
//
//go:embed *.schema.json
var Files embed.FS

// FileOf maps each config type to its schema file (specs/schemas/<file>).
var FileOf = map[string]string{
	gameconfig.TypeMatchmaking:  "matchmaking-config.schema.json",
	gameconfig.TypeRanking:      "ranking-rules.schema.json",
	gameconfig.TypeSeasons:      "season.schema.json",
	gameconfig.TypeTournamentTp: "tournament-template.schema.json",
	gameconfig.TypeChallengeTp:  "challenge-template.schema.json",
	gameconfig.TypeLeaderboards: "leaderboard-definition.schema.json",
	gameconfig.TypeRewards:      "competition-reward.schema.json",
}

var (
	once     sync.Once
	compiled map[string]*jsonschema.Schema
	errC     error
)

func load() (map[string]*jsonschema.Schema, error) {
	once.Do(func() {
		out := map[string]*jsonschema.Schema{}
		for typ, file := range FileOf {
			b, err := Files.ReadFile(file)
			if err != nil {
				errC = err
				return
			}
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
			if err != nil {
				errC = fmt.Errorf("configschema: parse %s: %w", file, err)
				return
			}
			c := jsonschema.NewCompiler()
			c.AssertFormat()
			url := "urn:eduworld:competition-config:" + typ
			if err := c.AddResource(url, doc); err != nil {
				errC = err
				return
			}
			s, err := c.Compile(url)
			if err != nil {
				errC = fmt.Errorf("configschema: compile %s: %w", file, err)
				return
			}
			out[typ] = s
		}
		compiled = out
	})
	return compiled, errC
}

// Validator implements port.ConfigSchemas.
type Validator struct{}

var _ port.ConfigSchemas = Validator{}

// Validate checks raw against the schema of configType; each violation names the failing field ("payload/…") without
// echoing its value.
func (Validator) Validate(configType string, raw []byte) ([]gameconfig.Violation, error) {
	all, err := load()
	if err != nil {
		return nil, err
	}
	s, ok := all[configType]
	if !ok {
		return nil, fmt.Errorf("configschema: no schema for %s", configType)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return []gameconfig.Violation{{Field: "payload", Code: gameconfig.CodeInvalid, Message: "not JSON"}}, nil //nolint:nilerr // a payload that is not JSON is a field violation (422), not an internal error
	}
	err = s.Validate(v)
	if err == nil {
		return nil, nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return nil, err
	}
	return violations(ve), nil
}

// violations lists the leaf errors by instance location (at most 50, sorted).
func violations(ve *jsonschema.ValidationError) []gameconfig.Violation {
	seen := map[string]string{}
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			path := "payload"
			if len(e.InstanceLocation) > 0 {
				path += "/" + strings.Join(e.InstanceLocation, "/")
			}
			if _, ok := seen[path]; !ok {
				seen[path] = kindOf(e)
			}
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]gameconfig.Violation, 0, len(paths))
	for i, p := range paths {
		if i == 50 {
			break
		}
		out = append(out, gameconfig.Violation{Field: p, Code: gameconfig.CodeInvalid, Message: seen[p]})
	}
	return out
}

// kindOf is a short value-free reason of a schema error (the keyword that failed).
func kindOf(e *jsonschema.ValidationError) string {
	if e.ErrorKind == nil {
		return "does not match the schema"
	}
	kw := e.ErrorKind.KeywordPath()
	if len(kw) == 0 {
		return "does not match the schema"
	}
	return "breaks the schema keyword " + strings.Join(kw, "/")
}
