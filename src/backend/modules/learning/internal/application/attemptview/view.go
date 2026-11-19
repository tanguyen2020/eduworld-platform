// Package attemptview builds the views of the Activity & Attempt Engine shared by its commands (the stored idempotent
// responses) and its queries (F030): render payloads without answers, attempts, item feedback after grading.
package attemptview

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/dto"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/activity"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/attempt"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/i18ntext"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
)

// ActivityBody decodes the activity fields of a version body (ActivityDefinitionInput minus the metadata columns).
type ActivityBody struct {
	ActivityType     string          `json:"activity_type"`
	Instructions     json.RawMessage `json:"instructions,omitempty"`
	Difficulty       *int            `json:"difficulty,omitempty"`
	TimeLimitSeconds *int            `json:"time_limit_seconds,omitempty"`
	RewardRuleID     *uuid.UUID      `json:"reward_rule_id,omitempty"`
	Config           json.RawMessage `json:"config,omitempty"`
	MathAssets       json.RawMessage `json:"math_assets,omitempty"`
}

// ParseBody decodes the body of an activity version.
func ParseBody(body json.RawMessage) ActivityBody {
	var a ActivityBody
	_ = json.Unmarshal(body, &a)
	return a
}

// Source is an activity version with its plugin and parsed config.
type Source struct {
	Doc     dto.Doc
	Version dto.DocVersion
	Body    ActivityBody
	Config  activity.Config
	Plugin  activity.Plugin
}

// DomainVersion is what the plugin reads of the version.
func (s Source) DomainVersion() activity.Version {
	return activity.Version{ActivityID: s.Doc.ID, VersionID: s.Version.ID, VersionNo: s.Version.VersionNo, Type: activity.Type(s.Body.ActivityType),
		Config: s.Config, TimeLimitSeconds: s.Body.TimeLimitSeconds, ObjectiveIDs: s.Version.ObjectiveIDs}
}

// TimeLimit is time_limit_seconds of the activity, or the timer of a timed quiz.
func (s Source) TimeLimit() *int {
	if s.Body.TimeLimitSeconds != nil {
		return s.Body.TimeLimitSeconds
	}
	if t := s.Config.Content.Timed; t != nil && t.TotalSeconds > 0 {
		n := t.TotalSeconds
		return &n
	}
	return nil
}

func plainString(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	return ""
}

// Render builds the ActivityRender of a version for a display language (no answer, scoring or option feedback).
func Render(src Source, reg *activity.Registry, want string) dto.ActivityRenderView {
	lang := src.Version.ContentLanguage
	title, served := i18ntext.Resolve(src.Version.Title, want, lang)
	r := dto.ActivityRenderView{ActivityID: src.Doc.ID, ActivityVersionID: src.Version.ID, Version: src.Version.VersionNo,
		ActivityType: src.Body.ActivityType, Title: plainString(title), TimeLimitSeconds: src.TimeLimit(),
		RenderPayload: src.Plugin.BuildRenderPayload(src.DomainVersion()), MinClientVersion: reg.MinClientVersion(src.Body.ActivityType),
		ContentLanguage: lang}
	if len(src.Body.MathAssets) > 0 && string(src.Body.MathAssets) != "null" {
		r.MathAssets = src.Body.MathAssets
	}
	if len(src.Body.Instructions) > 0 {
		ins, s := i18ntext.Resolve(src.Body.Instructions, want, lang)
		r.Instructions = plainString(ins)
		if s != want {
			served = s
		}
	}
	r.DisplayLanguage = dto.LocaleOf(served)
	r.TranslationMissing = served != want
	return r
}

// Items builds QuestionRender of the items (the render DTO plus its stem; never an answer).
func Items(items []attempt.Item, rendered []question.Rendered) []dto.QuestionRenderView {
	byID := map[uuid.UUID]question.Rendered{}
	for _, r := range rendered {
		byID[r.QuestionVersionID] = r
	}
	out := make([]dto.QuestionRenderView, 0, len(items))
	for _, it := range items {
		out = append(out, Item(it, byID[it.QuestionVersionID]))
	}
	return out
}

// Item builds the QuestionRender of one item.
func Item(it attempt.Item, r question.Rendered) dto.QuestionRenderView {
	payload := map[string]any{}
	for k, v := range r.Payload {
		payload[k] = v
	}
	if _, ok := payload["stem"]; !ok && r.Stem != nil {
		payload["stem"] = r.Stem
	}
	return dto.QuestionRenderView{Position: it.Position, QuestionVersionID: it.QuestionVersionID, Type: it.QuestionType,
		RenderPayload: payload, MaxScore: it.MaxScore}
}

// Attempt is the Attempt view of an attempt (server_time = now).
func Attempt(a attempt.Attempt, items []dto.QuestionRenderView, saved []dto.SavedResponseView, render *dto.ActivityRenderView,
	ev *dto.EvaluationView, now time.Time) dto.AttemptView {
	v := dto.AttemptView{AttemptID: a.ID, ActivityID: a.ActivityID, ActivityType: a.ActivityType, Mode: string(a.Mode), Status: string(a.Status),
		ContextType: a.ContextType, ContextID: a.ContextID, StartedAt: a.StartedAt, DeadlineAt: a.DeadlineAt, SubmittedAt: a.SubmittedAt,
		ServerTime: now, FeedbackTiming: string(a.Feedback), PausedAt: a.PausedAt, RecommendationID: a.RecommendationID,
		DisplayLanguage: dto.LocaleOf(a.DisplayLanguage), Items: items, SavedResponses: saved, Render: render, Evaluation: ev}
	if v.Items == nil {
		v.Items = []dto.QuestionRenderView{}
	}
	if v.SavedResponses == nil {
		v.SavedResponses = []dto.SavedResponseView{}
	}
	if a.ActivityID != nil {
		id := a.SourceVersionID
		v.ActivityVersionID = &id
	}
	return v
}

// OptionFeedback returns the AnswerFeedback of an option (nil without feedback, ACTIVITY_ENGINE §8.2).
func OptionFeedback(fbs map[string]question.OptionFeedback, optionID, lang string) *dto.AnswerFeedbackView {
	fb, ok := fbs[optionID]
	if !ok || len(fb.Feedback) == 0 {
		return nil
	}
	text := RichText(fb.Feedback, lang)
	if text == "" {
		return nil
	}
	return &dto.AnswerFeedbackView{OptionID: optionID, Text: Truncate(text, 2000)}
}

// SelectedOption is the option a stored response chose (single choice, or the first of a multiple choice), "" otherwise.
func SelectedOption(response json.RawMessage) string {
	var r struct {
		OptionID  string   `json:"option_id"`
		OptionIDs []string `json:"option_ids"`
	}
	if json.Unmarshal(response, &r) != nil {
		return ""
	}
	if r.OptionID != "" {
		return r.OptionID
	}
	if len(r.OptionIDs) > 0 {
		return r.OptionIDs[0]
	}
	return ""
}

// Truncate cuts s to n runes.
func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// RichText flattens a learning text (string, i18n object or richContent blocks) to plain text in lang.
func RichText(raw json.RawMessage, lang string) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	resolved, _ := i18ntext.Resolve(raw, lang, lang)
	var v any
	if json.Unmarshal(resolved, &v) != nil {
		return ""
	}
	var parts []string
	var rec func(x any)
	rec = func(x any) {
		switch t := x.(type) {
		case string:
			parts = append(parts, t)
		case map[string]any:
			if s, ok := t["text"].(string); ok {
				parts = append(parts, s)
				return
			}
			if b, ok := t["blocks"]; ok {
				rec(b)
				return
			}
			for _, k := range []string{lang, "vi", "en"} {
				if s, ok := t[k]; ok {
					rec(s)
					return
				}
			}
		case []any:
			for _, e := range t {
				rec(e)
			}
		}
	}
	rec(v)
	return strings.TrimSpace(strings.Join(parts, " "))
}
