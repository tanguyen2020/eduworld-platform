// Package render is the work queue between the worker of module game and the game-renderer container (F078 US5,
// FR-CMS-017, research R2): the worker enqueues one Job per (published version, 3D source asset, kind) on the
// JetStream work-queue stream GAME_RENDER_JOBS; the container (Godot headless, src/game/tools/thumbnail_renderer)
// renders it, uploads the PNG content-addressed to ew-media-public (media/<sha256>/<kind>.png) and answers one Result
// on the same stream; the worker records the result (media_assets + media_usages) in its database. The renderer never
// touches the database of module game (R13).
package render

import (
	"time"

	"github.com/google/uuid"
)

// Stream and subjects of the work queue (retention WorkQueue: each message is consumed by one consumer, whose filter
// subjects do not overlap).
const (
	Stream         = "GAME_RENDER_JOBS"
	SubjectAll     = "ew.job.game.render.>"
	SubjectRequest = "ew.job.game.render.request"
	SubjectResult  = "ew.job.game.render.result"
	// ConsumerRenderer is the durable pull consumer of the container; ConsumerResults the one of the worker.
	ConsumerRenderer = "game_renderer"
	ConsumerResults  = "game_render_results"
	// MaxAge bounds an unconsumed message (the job is re-enqueued by the retry sweep anyway).
	MaxAge = 24 * time.Hour
)

// Kinds of render (media_assets.kind of the result).
const (
	KindThumbnail = "render_thumbnail"
	KindTurntable = "render_turntable"
)

// Result statuses.
const (
	StatusDone   = "done"
	StatusFailed = "failed"
)

// Geometry of the outputs (DESIGN_SYSTEM §7): a 512×512 thumbnail; a turntable of 24 frames of 512×512 laid out as a
// sprite sheet of TurntableColumns columns (6 × 4 = 3072×2048 PNG).
const (
	Size             = 512
	TurntableFrames  = 24
	TurntableColumns = 6
	MaxErrorLen      = 1000
)

// Source is the 3D asset to render: a ready game media asset of kind model3d in ew-media-public.
type Source struct {
	AssetID uuid.UUID `json:"asset_id"`
	Bucket  string    `json:"bucket"`
	Key     string    `json:"key"`
	MIME    string    `json:"mime_type"`
	SHA256  string    `json:"sha256"`
}

// Job is one render request (subject SubjectRequest, Nats-Msg-Id = render_job_id + ":" + attempt).
type Job struct {
	RenderJobID uuid.UUID `json:"render_job_id"`
	Kind        string    `json:"kind"`
	Size        int       `json:"size"`
	Frames      int       `json:"frames"`
	Source      Source    `json:"source"`
	Attempt     int       `json:"attempt"`
}

// Output is the uploaded render: an immutable object media/<sha256>/<kind>.png of OutputBucket.
type Output struct {
	Bucket    string `json:"bucket"`
	Key       string `json:"key"`
	MIME      string `json:"mime_type"`
	SHA256    string `json:"sha256"`
	SizeBytes int64  `json:"size_bytes"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Frames    int    `json:"frames"`
}

// Result is the answer of the renderer (subject SubjectResult, Nats-Msg-Id = "result:" + render_job_id + ":" + attempt).
type Result struct {
	RenderJobID uuid.UUID `json:"render_job_id"`
	Attempt     int       `json:"attempt"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Output      *Output   `json:"output,omitempty"`
}

// OutputBucket is the bucket of the renders (platform game media, CDN public, ADR-0044).
const OutputBucket = "ew-media-public"

// OutputKey is the content-addressed key of a render.
func OutputKey(sha256, kind string) string { return "media/" + sha256 + "/" + kind + ".png" }
