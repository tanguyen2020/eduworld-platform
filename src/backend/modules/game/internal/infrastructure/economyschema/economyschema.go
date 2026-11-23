// Package economyschema checks the payloads of the F075 game content against the embedded copies of
// specs/schemas/item.schema.json, shop-item.schema.json, collection.schema.json and achievement.schema.json. The copies are
// kept equal to the specs by TestEmbeddedCopiesMatchSpecs (single source: specs/schemas, R33).
package economyschema

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/game/internal/application/port"
)

// Embedded copies of the specs.
var (
	//go:embed item.schema.json
	ItemJSON []byte
	//go:embed shop-item.schema.json
	ShopItemJSON []byte
	//go:embed collection.schema.json
	CollectionJSON []byte
	//go:embed achievement.schema.json
	AchievementJSON []byte
)

const (
	itemURL        = "urn:eduworld:schema:item:1"
	shopItemURL    = "urn:eduworld:schema:shop-item:1"
	collectionURL  = "urn:eduworld:schema:collection:1"
	achievementURL = "urn:eduworld:schema:achievement:1"
)

// ErrInvalid is a payload outside its JSON Schema; Error.Pointers lists the failing JSON pointers (no values).
var ErrInvalid = errors.New("outside the JSON Schema")

// Error is a schema failure with the JSON pointers of the failing values.
type Error struct {
	Schema   string
	Pointers []string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.Schema, ErrInvalid, strings.Join(e.Pointers, ", "))
}
func (e *Error) Unwrap() error { return ErrInvalid }

var (
	once       sync.Once
	compiled   map[string]*jsonschema.Schema
	errCompile error
)

func compile() error {
	once.Do(func() {
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		sources := map[string][]byte{itemURL: ItemJSON, shopItemURL: ShopItemJSON, collectionURL: CollectionJSON, achievementURL: AchievementJSON}
		for url, raw := range sources {
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
			if err != nil {
				errCompile = fmt.Errorf("economyschema: parse %s: %w", url, err)
				return
			}
			if err := c.AddResource(url, doc); err != nil {
				errCompile = err
				return
			}
		}
		compiled = map[string]*jsonschema.Schema{}
		for url := range sources {
			s, err := c.Compile(url)
			if err != nil {
				errCompile = err
				return
			}
			compiled[url] = s
		}
	})
	return errCompile
}

func validate(url, name string, raw []byte) error {
	if err := compile(); err != nil {
		return err
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return &Error{Schema: name, Pointers: []string{"/"}}
	}
	if err := compiled[url].Validate(v); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return &Error{Schema: name, Pointers: pointers(ve)}
		}
		return &Error{Schema: name, Pointers: []string{"/"}}
	}
	return nil
}

// Item, ShopItem, Collection and Achievement implement port.PayloadValidator for a stored version of their type.
type (
	Item        struct{}
	ShopItem    struct{}
	Collection  struct{}
	Achievement struct{}
)

var (
	_ port.PayloadValidator = Item{}
	_ port.PayloadValidator = ShopItem{}
	_ port.PayloadValidator = Collection{}
	_ port.PayloadValidator = Achievement{}
)

// Validate implements port.PayloadValidator.
func (Item) Validate(raw []byte) error { return validate(itemURL, "item.schema.json", raw) }

// Validate implements port.PayloadValidator.
func (ShopItem) Validate(raw []byte) error {
	return validate(shopItemURL, "shop-item.schema.json", raw)
}

// Validate implements port.PayloadValidator.
func (Collection) Validate(raw []byte) error {
	return validate(collectionURL, "collection.schema.json", raw)
}

// Validate implements port.PayloadValidator.
func (Achievement) Validate(raw []byte) error {
	return validate(achievementURL, "achievement.schema.json", raw)
}

func pointers(ve *jsonschema.ValidationError) []string {
	seen := map[string]bool{}
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			seen["/"+strings.Join(e.InstanceLocation, "/")] = true
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	out := make([]string, 0, len(seen))
	for p := range seen {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
