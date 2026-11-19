package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port/write"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/mathx"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/question"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/observability"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// MathMetrics counts refused formulas (eduworld_math_render_failed_total{reason}); nil = not counted.
type MathMetrics interface {
	MathFailed(reason string)
}

// MathAssets is the formula step of the validation pipeline of a draft save (F027 US3, ADR-0055 §4, §7): the LaTeX
// allowlist, the sanitized SVG sent by the portal stored once at math/<sha256>.svg (ew-media-public, immutable) with
// its row in content.math_svgs, and the stored form {latex, svg_asset, speech_text} of the version.
type MathAssets struct {
	Store   write.MathStore
	Storage port.ObjectStorage
	Limits  mathx.Limits // zero = mathx.DefaultLimits
	Clock   func() time.Time
	Metrics MathMetrics
}

// mathInput is one math_assets item of an authoring request.
type mathInput struct {
	Latex      string  `json:"latex"`
	SVG        *string `json:"svg,omitempty"`
	SVGAsset   string  `json:"svg_asset,omitempty"`
	SpeechText string  `json:"speech_text"`
}

func (m MathAssets) limits() mathx.Limits {
	if m.Limits == (mathx.Limits{}) {
		return mathx.DefaultLimits
	}
	return m.Limits
}

// mathFailed is 422 MATH_RENDER_FAILED with the JSON pointer of the formula (F027-AC-12, AC-13).
func (m MathAssets) mathFailed(pointer, reason, metric string) error {
	if m.Metrics != nil {
		m.Metrics.MathFailed(metric)
	}
	if utf8.RuneCountInString(reason) > 500 {
		reason = string([]rune(reason)[:500])
	}
	pe := problem.New(problem.MathRenderFailed, "a formula cannot be rendered safely")
	pe.AddField(pointer, problem.SubInvalid, reason)
	return pe
}

// Resolve validates the math_assets of a version (raw authoring JSON) and returns their stored form: every LaTeX
// passes the allowlist; an item with svg is sanitized and stored (an identical SVG is reused, never rewritten); an
// item without svg must name an SVG already stored. A version keeps the SVG it was saved with: a later MathJax
// changes only new saves (F027-AC-15).
func (m MathAssets) Resolve(ctx context.Context, raw json.RawMessage) (stored json.RawMessage, out []question.MathAsset, err error) {
	if isNull(raw) {
		return nil, nil, nil
	}
	ctx, done := observability.UseCase(ctx, "math.validate")
	defer func() { done(err) }()
	var in []mathInput
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&in); err != nil {
		return nil, nil, problem.Invalid("/math_assets", problem.SubInvalid, "must be a list of {latex, svg, speech_text}")
	}
	lim := m.limits()
	at := now(m.Clock)
	out = make([]question.MathAsset, 0, len(in))
	for i, it := range in {
		ptr := "/math_assets/" + strconv.Itoa(i)
		if err := mathx.ValidateLatex(it.Latex, lim); err != nil {
			var me mathx.Error
			if errors.As(err, &me) {
				return nil, nil, m.mathFailed(ptr+"/latex", me.Reason, "latex")
			}
			return nil, nil, err
		}
		var key string
		switch {
		case it.SVG != nil:
			clean, err := mathx.SanitizeSVG([]byte(*it.SVG), mathx.MathProfile, lim.SVGMaxBytes)
			if err != nil {
				var me mathx.Error
				if errors.As(err, &me) {
					return nil, nil, m.mathFailed(ptr+"/svg", me.Reason, "svg")
				}
				return nil, nil, err
			}
			sha := mathx.Hash(clean)
			key = mathx.Key(sha)
			if err := m.store(ctx, sha, key, clean, it, at); err != nil {
				return nil, nil, err
			}
		case it.SVGAsset != "":
			sha := ""
			if mathx.SVGKeyPattern.MatchString(it.SVGAsset) {
				sha = it.SVGAsset[len("math/") : len(it.SVGAsset)-len(".svg")]
			}
			_, ok, err := m.Store.MathSvg(ctx, sha)
			if err != nil {
				return nil, nil, err
			}
			if sha == "" || !ok {
				return nil, nil, m.mathFailed(ptr+"/svg_asset", "the SVG of the formula is not stored; send svg", "svg_missing")
			}
			key = it.SVGAsset
		default:
			return nil, nil, m.mathFailed(ptr+"/svg", "the rendered SVG of the formula is missing", "svg_missing")
		}
		out = append(out, question.MathAsset{Latex: it.Latex, SVGAsset: key, SpeechText: it.SpeechText})
	}
	b, err := json.Marshal(out)
	if err != nil {
		return nil, nil, err
	}
	return b, out, nil
}

// store writes the object first (content-addressed: a retry rewrites the same bytes), then its row.
func (m MathAssets) store(ctx context.Context, sha, key string, clean []byte, it mathInput, at time.Time) error {
	if _, ok, err := m.Store.MathSvg(ctx, sha); err != nil || ok {
		return err
	}
	if err := m.Storage.Put(ctx, pkgmedia.BucketPublic, key, bytes.NewReader(clean), int64(len(clean)), "image/svg+xml",
		pkgmedia.ImmutableCacheControl); err != nil {
		return problem.New(problem.Unavailable, "object storage unavailable").Retry().Wrap(err)
	}
	return m.Store.InsertMathSvg(ctx, write.MathSvg{SHA256: sha, StorageKey: key, Latex: it.Latex, SpeechText: it.SpeechText,
		SizeBytes: len(clean), SanitizedAt: at})
}
