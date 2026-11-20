package qbschema

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// LearningEventSchemaJSON is the embedded copy of specs/schemas/learning-event.schema.json (EVENT_DESIGN §5, F031-AC-01).
//
//go:embed learning-event.schema.json
var LearningEventSchemaJSON []byte

const learningEventURL = "https://schemas.eduworld.example/learning-event.v1.schema.json"

// ErrLearningEventInvalid is returned for a learning event outside learning-event.schema.json (F031-AC-05): an unknown
// payload field (a name, an email, a position in the world, a chat text…) or a wrong envelope is refused at write time.
var ErrLearningEventInvalid = errors.New("learning event outside learning-event.schema.json")

var (
	leOnce   sync.Once
	leSchema *jsonschema.Schema
	leErr    error
)

func learningEventSchema() (*jsonschema.Schema, error) {
	leOnce.Do(func() {
		c := jsonschema.NewCompiler()
		c.AssertFormat()
		for url, b := range map[string][]byte{learningEventURL: LearningEventSchemaJSON, questionURL: QuestionSchemaJSON, enumsURL: EnumsSchemaJSON} {
			doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
			if err != nil {
				leErr = fmt.Errorf("qbschema: parse %s: %w", url, err)
				return
			}
			if err := c.AddResource(url, doc); err != nil {
				leErr = err
				return
			}
		}
		leSchema, leErr = c.Compile(learningEventURL)
	})
	return leSchema, leErr
}

// ValidateLearningEvent checks a complete learning event document (envelope + payload) against the schema. The error
// names the JSON pointers only, never the values (R14: a refused payload may hold personal data).
func ValidateLearningEvent(doc []byte) error {
	s, err := learningEventSchema()
	if err != nil {
		return err
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return fmt.Errorf("%w: not JSON", ErrLearningEventInvalid)
	}
	if err := s.Validate(v); err != nil {
		var ve *jsonschema.ValidationError
		if errors.As(err, &ve) {
			return fmt.Errorf("%w: %s", ErrLearningEventInvalid, strings.Join(pointers(ve), ", "))
		}
		return fmt.Errorf("%w", ErrLearningEventInvalid)
	}
	return nil
}

// pointers lists the instance locations of the leaf errors (no values).
func pointers(ve *jsonschema.ValidationError) []string {
	seen := map[string]bool{}
	var out []string
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			p := "/" + strings.Join(e.InstanceLocation, "/")
			if !seen[p] {
				seen[p] = true
				out = append(out, p)
			}
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}
