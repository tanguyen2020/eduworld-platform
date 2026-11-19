// Package qbschema validates question authoring data and student responses against specs/schemas/question.schema.json
// (F022 T006): the schema and the shared enums are embedded copies kept equal to the specs by TestEmbeddedSchemas.
package qbschema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// QuestionSchemaJSON and EnumsSchemaJSON are the embedded copies of specs/schemas.
var (
	//go:embed question.schema.json
	QuestionSchemaJSON []byte
	//go:embed enums.schema.json
	EnumsSchemaJSON []byte
)

const (
	questionURL = "https://schemas.eduworld.vn/content/question.schema.json"
	enumsURL    = "https://schemas.eduworld.vn/common/enums.schema.json"
)

var typeDefs = map[string]string{"single_choice": "singleChoice", "multiple_choice": "multipleChoice", "true_false": "trueFalse",
	"fill_in_blank": "fillBlank", "short_answer": "shortAnswer", "numeric": "numeric", "matching": "matching", "ordering": "ordering",
	"drag_drop": "dragDrop", "image_based": "imageBased", "interactive": "interactive"}

var responseDefs = map[string]string{"single_choice": "singleChoiceResponse", "multiple_choice": "multipleChoiceResponse",
	"true_false": "trueFalseResponse", "fill_in_blank": "fillInBlankResponse", "short_answer": "shortAnswerResponse", "numeric": "numericResponse",
	"matching": "matchingResponse", "ordering": "orderingResponse", "drag_drop": "dragDropResponse", "image_based": "imageBasedResponse",
	"interactive": "interactiveResponse"}

// Validator implements port.QuestionSchema.
type Validator struct {
	types, responses       map[string]*jsonschema.Schema
	rich, scoring, mathDef *jsonschema.Schema
}

var _ port.QuestionSchema = (*Validator)(nil)

// New compiles the embedded schemas.
func New() (*Validator, error) {
	c := jsonschema.NewCompiler()
	for url, b := range map[string][]byte{questionURL: QuestionSchemaJSON, enumsURL: EnumsSchemaJSON} {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("qbschema: parse %s: %w", url, err)
		}
		if err := c.AddResource(url, doc); err != nil {
			return nil, err
		}
	}
	compile := func(def string) (*jsonschema.Schema, error) { return c.Compile(questionURL + "#/$defs/" + def) }
	v := &Validator{types: map[string]*jsonschema.Schema{}, responses: map[string]*jsonschema.Schema{}}
	for t, def := range typeDefs {
		s, err := compile(def)
		if err != nil {
			return nil, fmt.Errorf("qbschema: %s: %w", def, err)
		}
		v.types[t] = s
		if v.responses[t], err = compile(responseDefs[t]); err != nil {
			return nil, err
		}
	}
	var err error
	if v.rich, err = compile("richContent"); err != nil {
		return nil, err
	}
	if v.scoring, err = compile("scoringRule"); err != nil {
		return nil, err
	}
	if v.mathDef, err = compile("mathDisplay"); err != nil {
		return nil, err
	}
	return v, nil
}

func load(raw json.RawMessage) (any, error) {
	return jsonschema.UnmarshalJSON(bytes.NewReader(raw))
}

// ValidateAuthoring implements port.QuestionSchema: the type branch (render_payload + answer_key), the scoring rule,
// stem/explanation/hints as richContent; violations are JSON pointers.
func (v *Validator) ValidateAuthoring(a port.QuestionDoc) contentmodel.Violations {
	var out contentmodel.Violations
	check := func(s *jsonschema.Schema, prefix string, raw json.RawMessage) {
		if len(raw) == 0 {
			return
		}
		doc, err := load(raw)
		if err != nil {
			out = append(out, contentmodel.Violation{Field: prefix, Reason: "is not JSON"})
			return
		}
		collect(s.Validate(doc), prefix, &out)
	}
	s, ok := v.types[a.Type]
	if !ok {
		return contentmodel.Violations{{Field: "/type", Reason: "is not a question type"}}
	}
	branch, _ := json.Marshal(map[string]any{"type": a.Type, "render_payload": a.RenderPayload, "answer_key": a.AnswerKey})
	check(s, "", branch)
	check(v.scoring, "/scoring_rule", a.ScoringRule)
	check(v.rich, "/stem", a.Stem)
	check(v.rich, "/explanation", a.Explanation)
	for i, h := range a.Hints {
		check(v.rich, fmt.Sprintf("/hints/%d", i), h)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

// ValidateResponse implements port.QuestionSchema ($defs.<type>Response).
func (v *Validator) ValidateResponse(typ string, response json.RawMessage) error {
	s, ok := v.responses[typ]
	if !ok {
		return errInvalid
	}
	doc, err := load(response)
	if err != nil {
		return errInvalid
	}
	if s.Validate(doc) != nil {
		return errInvalid
	}
	return nil
}

var errInvalid = errors.New("qbschema: invalid response")

func collect(err error, prefix string, out *contentmodel.Violations) {
	if err == nil {
		return
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		*out = append(*out, contentmodel.Violation{Field: prefix, Reason: "is invalid"})
		return
	}
	var rec func(*jsonschema.ValidationError)
	rec = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			loc := prefix
			if len(e.InstanceLocation) > 0 {
				loc += "/" + strings.Join(e.InstanceLocation, "/")
			}
			msg := e.ErrorKind.LocalizedString(printer)
			if len([]rune(msg)) > 200 {
				msg = string([]rune(msg)[:200])
			}
			*out = append(*out, contentmodel.Violation{Field: loc, Reason: msg})
			return
		}
		for _, c := range e.Causes {
			rec(c)
		}
	}
	rec(ve)
}

var printer = message.NewPrinter(language.English)
