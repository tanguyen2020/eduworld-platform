package qbschema

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

// Embedded copies of the content schemas of F024 (kept equal to specs/schemas by TestEmbeddedSchemas).
var (
	//go:embed question-set.schema.json
	QuestionSetSchemaJSON []byte
	//go:embed assessment-blueprint.schema.json
	BlueprintSchemaJSON []byte
	//go:embed lesson.schema.json
	LessonSchemaJSON []byte
)

const contentBase = "https://schemas.eduworld.vn/content/"

// Wrapper schemas: the API sends the type-specific part of a version (definition, blocks); they reuse the
// definitions of the published schemas.
var wrappers = map[string]string{
	"set-definition": `{"$id":"` + contentBase + `wrap/set-definition.json","type":"object","required":["kind","purpose"],
	 "properties":{"kind":{"$ref":"../question-set.schema.json#/properties/kind"},"purpose":{"$ref":"../question-set.schema.json#/properties/purpose"},
	 "items":{"$ref":"../question-set.schema.json#/properties/items"},"selection_rules":{"$ref":"../question-set.schema.json#/properties/selection_rules"},
	 "shuffle_items":{"type":"boolean"},"shuffle_options":{"type":"boolean"},"estimated_duration_sec":{"type":"integer","minimum":1}},
	 "additionalProperties":false}`,
	"blueprint-definition": `{"$id":"` + contentBase + `wrap/blueprint-definition.json","type":"object",
	 "required":["duration_sec","sections","total_points","attempt_policy","result_release"],
	 "properties":{"duration_sec":{"$ref":"../assessment-blueprint.schema.json#/properties/duration_sec"},
	 "sections":{"$ref":"../assessment-blueprint.schema.json#/properties/sections"},
	 "total_points":{"$ref":"../assessment-blueprint.schema.json#/properties/total_points"},
	 "attempt_policy":{"$ref":"../assessment-blueprint.schema.json#/properties/attempt_policy"},
	 "result_release":{"$ref":"../assessment-blueprint.schema.json#/properties/result_release"},
	 "variants":{"$ref":"../assessment-blueprint.schema.json#/properties/variants"},
	 "grading_scale":{"$ref":"../assessment-blueprint.schema.json#/properties/grading_scale"},
	 "shuffle_options":{"type":"boolean"},"navigation":{"$ref":"../assessment-blueprint.schema.json#/properties/navigation"},
	 "instructions":{"$ref":"../assessment-blueprint.schema.json#/properties/instructions"}},
	 "additionalProperties":false}`,
}

// DocValidator implements port.ContentSchema (F024: question set definitions, lesson/material blocks, blueprint
// definitions).
type DocValidator struct {
	set, blueprint, blocks *jsonschema.Schema
}

var _ port.ContentSchema = (*DocValidator)(nil)

// NewDocs compiles the content schemas.
func NewDocs() (*DocValidator, error) {
	c := jsonschema.NewCompiler()
	res := map[string][]byte{questionURL: QuestionSchemaJSON, enumsURL: EnumsSchemaJSON, contentBase + "question-set.schema.json": QuestionSetSchemaJSON,
		contentBase + "assessment-blueprint.schema.json": BlueprintSchemaJSON, contentBase + "lesson.schema.json": LessonSchemaJSON}
	for name, w := range wrappers {
		res[contentBase+"wrap/"+name+".json"] = []byte(w)
	}
	for url, b := range res {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("qbschema: parse %s: %w", url, err)
		}
		if err := c.AddResource(url, doc); err != nil {
			return nil, err
		}
	}
	v := &DocValidator{}
	var err error
	if v.set, err = c.Compile(contentBase + "wrap/set-definition.json"); err != nil {
		return nil, fmt.Errorf("qbschema: set: %w", err)
	}
	if v.blueprint, err = c.Compile(contentBase + "wrap/blueprint-definition.json"); err != nil {
		return nil, fmt.Errorf("qbschema: blueprint: %w", err)
	}
	if v.blocks, err = c.Compile(contentBase + "lesson.schema.json#/properties/blocks"); err != nil {
		return nil, fmt.Errorf("qbschema: blocks: %w", err)
	}
	return v, nil
}

func (v *DocValidator) check(s *jsonschema.Schema, prefix string, raw json.RawMessage) contentmodel.Violations {
	var out contentmodel.Violations
	doc, err := load(raw)
	if err != nil {
		return contentmodel.Violations{{Field: prefix, Reason: "is not JSON"}}
	}
	collect(s.Validate(doc), prefix, &out)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}

// ValidateSetDefinition implements port.ContentSchema.
func (v *DocValidator) ValidateSetDefinition(def json.RawMessage) contentmodel.Violations {
	return v.check(v.set, "/definition", def)
}

// ValidateBlueprintDefinition implements port.ContentSchema.
func (v *DocValidator) ValidateBlueprintDefinition(def json.RawMessage) contentmodel.Violations {
	return v.check(v.blueprint, "/definition", def)
}

// ValidateBlocks implements port.ContentSchema.
func (v *DocValidator) ValidateBlocks(blocks json.RawMessage) contentmodel.Violations {
	return v.check(v.blocks, "/blocks", blocks)
}
