package command

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// bodyString returns a string member of the body of a version ("" when absent).
func bodyString(v dto.DocVersion, key string) string {
	var s string
	_ = json.Unmarshal(bodyOf(v)[key], &s)
	return s
}

// configProblem is 422 ACTIVITY_CONFIG_INVALID with the JSON pointers of the violations (F030-AC-21).
func configProblem(vs contentmodel.Violations) error {
	pe := &problem.Error{Code: problem.ActivityConfigInvalid, Detail: "the config does not match the schema of the activity type"}
	for _, v := range vs {
		sub := problem.SubInvalid
		if v.Required {
			sub = problem.SubRequired
		}
		pe.AddField(v.Field, sub, truncate(v.Reason, 500))
	}
	return pe
}

// ActivityPlugin resolves the plugin of an activity version (422 ACTIVITY_TYPE_NOT_SUPPORTED outside the release or
// without plugin, F030-AC-20).
func ActivityPlugin(reg *activity.Registry, activityType string) (activity.Plugin, error) {
	p, err := reg.Resolve(activityType)
	if errors.Is(err, activity.ErrNotSupported) {
		return nil, problem.New(problem.ActivityTypeNotSupported, "activity_type "+truncate(activityType, 32)+" has no plugin in this release")
	}
	return p, err
}

// prepareActivity checks an activity version on every draft save (and again before publish through the readiness of
// the workflow): the type has a plugin, the config matches the JSON Schema of the type and the plugin rules, fixed
// questions are visible to the owner, published and of an allowed type; the curriculum version is the one of the
// objectives (ActivityDefinitionInput has no curriculum_version_id).
func (h Docs) prepareActivity(ctx context.Context, v *dto.DocVersion, body map[string]json.RawMessage) error {
	var typ string
	if err := json.Unmarshal(body["activity_type"], &typ); err != nil || typ == "" {
		return problem.Invalid("/activity_type", problem.SubRequired, "is required")
	}
	p, err := ActivityPlugin(h.Activities, typ)
	if err != nil {
		return err
	}
	raw, ok := body["config"]
	if !ok || isNull(raw) {
		return configProblem(contentmodel.Violations{{Field: "/config", Reason: "is required", Required: true}})
	}
	if h.ActivitySchema == nil {
		return problem.New(problem.Internal, "no activity schema validator")
	}
	if vs := h.ActivitySchema.ValidateConfig(typ, raw); len(vs) > 0 {
		return configProblem(vs)
	}
	cfg, err := activity.ParseConfig(raw)
	if err != nil {
		return configProblem(contentmodel.Violations{{Field: "/config", Reason: "is invalid"}})
	}
	if vs := p.ValidateConfig(cfg); len(vs) > 0 {
		return configProblem(vs)
	}
	if len(v.ObjectiveIDs) == 0 {
		return problem.Invalid("/objective_ids", problem.SubRequired, "needs at least one learning objective")
	}
	if v.CurriculumVersionID == nil {
		o, err := h.Curriculum.Objective(ctx, v.ObjectiveIDs[0])
		if err != nil {
			return problem.Invalid("/objective_ids/0", problem.SubInvalid, "is not a visible learning objective")
		}
		cv := o.VersionID
		v.CurriculumVersionID = &cv
	}
	allowed := p.AllowedQuestionTypes(cfg)
	for i, ref := range cfg.Content.QuestionSource.Questions {
		ptr := "/config/content/question_source/questions/" + strconv.Itoa(i) + "/question_version_id"
		qv, err := h.checkQuestion(ctx, v.Owner, ref.QuestionVersionID, ptr)
		if err != nil {
			return err
		}
		if qv.Status != contentmodel.StatusPublished {
			return configProblem(contentmodel.Violations{{Field: ptr, Reason: "is not a published question version"}})
		}
		if !slices.Contains(allowed, qv.Type) {
			return configProblem(contentmodel.Violations{{Field: ptr, Reason: "has a question type not allowed by the activity"}})
		}
	}
	// Media activities (F065-AC-01, F066-AC-01): every embedded question is a published, auto-graded question version;
	// the times of an audio lesson are checked against the probed duration of its audio.
	if tl, ok := p.(activity.Timeline); ok {
		dv := activity.Version{Type: activity.Type(typ), Config: cfg}
		if err := h.checkMedia(ctx, p, tl, dv); err != nil {
			return err
		}
		cues, err := tl.CueQuestions(dv)
		if err != nil {
			return configProblem(contentmodel.Violations{{Field: "/config/content", Reason: "cannot be read"}})
		}
		list := "embedded_questions"
		if typ == string(activity.AudioLesson) {
			list = "questions"
		}
		for i, id := range cues {
			ptr := "/config/content/" + list + "/" + strconv.Itoa(i) + "/question_version_id"
			qv, err := h.checkQuestion(ctx, v.Owner, id, ptr)
			if err != nil {
				return err
			}
			if qv.Status != contentmodel.StatusPublished || !slices.Contains(allowed, qv.Type) {
				return configProblem(contentmodel.Violations{{Field: ptr, Reason: "is not a published, auto-graded question version"}})
			}
		}
	}
	if id := cfg.Content.QuestionSource.QuestionSetID; id != nil {
		if _, err := h.Store.Doc(ctx, contentmodel.TypeQuestionSet, *id); err != nil {
			return configProblem(contentmodel.Violations{{Field: "/config/content/question_source/question_set_id", Reason: "is not a visible question set"}})
		}
	}
	if raw, ok := body["time_limit_seconds"]; ok && !isNull(raw) {
		var n int
		if err := json.Unmarshal(raw, &n); err != nil || n < 10 || n > 14400 {
			return problem.Invalid("/time_limit_seconds", problem.SubInvalid, "must be between 10 and 14400")
		}
	}
	return nil
}

// checkMedia runs the rules of a MediaCheck plugin with the probed duration of its main media (0 when the media store
// is absent or the asset has no duration yet: only the relative rules apply).
func (h Docs) checkMedia(ctx context.Context, p activity.Plugin, tl activity.Timeline, v activity.Version) error {
	mc, ok := p.(activity.MediaCheck)
	if !ok {
		return nil
	}
	var duration int64
	if ids, err := tl.MediaAssetIDs(v); err == nil && len(ids) > 0 && h.Media != nil {
		assets, err := h.Media.Assets(ctx, ids[:1])
		if err != nil {
			return err
		}
		if a, ok := assets[ids[0]]; ok && a.DurationMS != nil {
			duration = int64(*a.DurationMS)
		}
	}
	if vs := mc.ValidateMedia(v.Config, duration); len(vs) > 0 {
		return configProblem(vs)
	}
	return nil
}
