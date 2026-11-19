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

// Embedded copies of the activity schemas of F030 (kept equal to specs/schemas/activities by TestEmbeddedSchemas).
var (
	//go:embed activity-base.schema.json
	ActivityBaseSchemaJSON []byte
	//go:embed quiz.schema.json
	QuizSchemaJSON []byte
	//go:embed interactive-exercise.schema.json
	InteractiveExerciseSchemaJSON []byte
)

const activitiesBase = contentBase + "activities/"

// activitySchemas maps an activity_type with a plugin to its schema file (ACTIVITY_ENGINE §3).
var activitySchemas = map[string]struct {
	file string
	src  []byte
}{
	"quiz":                 {"quiz.schema.json", QuizSchemaJSON},
	"interactive_exercise": {"interactive-exercise.schema.json", InteractiveExerciseSchemaJSON},
}

// configWrapper is the config of an ActivityDefinitionInput (content, interaction, evaluation, feedback, completion_rule)
// checked with the definitions of the type schema: the other fields of the published schema are metadata columns.
func configWrapper(typ, file string) string {
	return `{"$id":"` + contentBase + `wrap/activity-config-` + typ + `.json","type":"object",
	 "required":["content","evaluation","feedback","completion_rule"],
	 "properties":{"content":{"$ref":"../activities/` + file + `#/properties/content"},
	 "interaction":{"$ref":"../activities/` + file + `#/properties/interaction"},
	 "evaluation":{"$ref":"../activities/activity-base.schema.json#/$defs/evaluation"},
	 "feedback":{"$ref":"../activities/activity-base.schema.json#/$defs/feedback"},
	 "completion_rule":{"$ref":"../activities/activity-base.schema.json#/$defs/completionRule"}},
	 "additionalProperties":false}`
}

// ActivityValidator implements port.ActivitySchema (ADR-0048: the JSON Schema of each activity_type).
type ActivityValidator struct {
	configs map[string]*jsonschema.Schema
}

var _ port.ActivitySchema = (*ActivityValidator)(nil)

// NewActivities compiles the config schemas of the activity types with a plugin.
func NewActivities() (*ActivityValidator, error) {
	c := jsonschema.NewCompiler()
	res := map[string][]byte{enumsURL: EnumsSchemaJSON, questionURL: QuestionSchemaJSON, activitiesBase + "activity-base.schema.json": ActivityBaseSchemaJSON}
	for typ, s := range activitySchemas {
		res[activitiesBase+s.file] = s.src
		res[contentBase+"wrap/activity-config-"+typ+".json"] = []byte(configWrapper(typ, s.file))
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
	v := &ActivityValidator{configs: map[string]*jsonschema.Schema{}}
	for typ := range activitySchemas {
		s, err := c.Compile(contentBase + "wrap/activity-config-" + typ + ".json")
		if err != nil {
			return nil, fmt.Errorf("qbschema: activity %s: %w", typ, err)
		}
		v.configs[typ] = s
	}
	return v, nil
}

// SchemaURL returns the $id of the schema of an activity type ("" without schema).
func SchemaURL(typ string) string {
	s, ok := activitySchemas[typ]
	if !ok {
		return ""
	}
	return activitiesBase + s.file
}

// ValidateConfig implements port.ActivitySchema: violations are JSON pointers under /config.
func (v *ActivityValidator) ValidateConfig(typ string, config json.RawMessage) contentmodel.Violations {
	s, ok := v.configs[typ]
	if !ok {
		return contentmodel.Violations{{Field: "/activity_type", Reason: "has no schema"}}
	}
	doc, err := load(config)
	if err != nil {
		return contentmodel.Violations{{Field: "/config", Reason: "is not JSON"}}
	}
	var out contentmodel.Violations
	collect(s.Validate(doc), "/config", &out)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Field < out[j].Field })
	return out
}
