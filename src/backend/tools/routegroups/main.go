// Command routegroups extracts the prefix → service groups of the APISIX route-as-code file
// (deploy/apisix/routes/routes.yaml, ADR-0074 §5) into a JSON document embedded by module identity: the
// integration overview of the platform admin (getIntegrationOverview, OQ F059-12) shows them without core-api
// reading a deployment file at run time.
//
//	go run ./tools/routegroups           # write modules/identity/internal/infrastructure/integration/route_groups.json
//	go run ./tools/routegroups --check   # fail when the embedded groups differ from routes.yaml
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Out is the embedded document, relative to the Go module root.
const Out = "modules/identity/internal/infrastructure/integration/route_groups.json"

// Group is one prefix → service group (IntegrationOverview.apisix.route_groups[]).
type Group struct {
	Prefix  string `json:"prefix"`
	Gateway string `json:"gateway"`
	Service string `json:"service"`
}

func main() {
	check := flag.Bool("check", false, "fail when the generated groups are stale")
	root := flag.String("root", ".", "Go module root (src/backend)")
	flag.Parse()
	want, err := Render(filepath.Join(*root, "..", "..", "deploy", "apisix", "routes", "routes.yaml"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "routegroups:", err)
		os.Exit(1)
	}
	path := filepath.Join(*root, Out)
	if *check {
		cur, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(bytes.ReplaceAll(cur, []byte("\r\n"), []byte("\n")), want) {
			fmt.Fprintln(os.Stderr, "routegroups: stale", path, "(run go run ./tools/routegroups)")
			os.Exit(1)
		}
		return
	}
	if err := os.WriteFile(path, want, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "routegroups:", err)
		os.Exit(1)
	}
}

// variable is a ${VAR} of the routes file (values of the environment, never needed for the groups).
var variable = regexp.MustCompile(`\$\{[A-Z0-9_]+\}`)

// Render returns the groups of the routes file as indented JSON: one group per URI and gateway (a route of
// gateway "both" gives a public and an internal group), the trailing wildcard removed, sorted.
func Render(routes string) ([]byte, error) {
	raw, err := os.ReadFile(routes)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Routes []struct {
			ID       string   `yaml:"id"`
			Gateway  string   `yaml:"gateway"`
			URIs     []string `yaml:"uris"`
			Upstream string   `yaml:"upstream"`
		} `yaml:"routes"`
	}
	if err := yaml.Unmarshal(variable.ReplaceAll(raw, []byte("0")), &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", routes, err)
	}
	seen := map[Group]bool{}
	out := []Group{}
	for _, r := range doc.Routes {
		var gateways []string
		switch r.Gateway {
		case "public", "internal":
			gateways = []string{r.Gateway}
		case "both":
			gateways = []string{"public", "internal"}
		default:
			return nil, fmt.Errorf("route %s: gateway %q", r.ID, r.Gateway)
		}
		if r.Upstream == "" || len(r.Upstream) > 64 {
			return nil, fmt.Errorf("route %s: upstream %q", r.ID, r.Upstream)
		}
		for _, uri := range r.URIs {
			prefix := strings.TrimSuffix(uri, "*")
			if prefix == "" || len(prefix) > 256 {
				return nil, fmt.Errorf("route %s: uri %q", r.ID, uri)
			}
			for _, gw := range gateways {
				g := Group{Prefix: prefix, Gateway: gw, Service: r.Upstream}
				if !seen[g] {
					seen[g] = true
					out = append(out, g)
				}
			}
		}
	}
	if len(out) > 100 {
		return nil, fmt.Errorf("%d route groups (IntegrationOverview allows 100)", len(out))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Gateway != out[j].Gateway {
			return out[i].Gateway > out[j].Gateway // public first
		}
		return out[i].Prefix < out[j].Prefix
	})
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}
