package port

import (
	"context"

	"github.com/google/uuid"
)

// RenderRequest is one render job for the game-renderer container (F078 US5; infrastructure/render.Job on the wire).
type RenderRequest struct {
	RenderJobID uuid.UUID
	Kind        string // render_thumbnail, render_turntable
	Frames      int
	Attempt     int
	SourceID    uuid.UUID
	Bucket      string
	Key         string
	MIME        string
	SHA256      string
}

// RenderQueue enqueues render jobs on the JetStream work queue GAME_RENDER_JOBS (at least once; the job id dedupes).
type RenderQueue interface {
	Enqueue(ctx context.Context, r RenderRequest) error
}

// RenderOutcome is a result of the renderer.
type RenderOutcome struct {
	RenderJobID uuid.UUID
	Attempt     int
	Done        bool
	Error       string
	Bucket      string
	Key         string
	MIME        string
	SHA256      string
	SizeBytes   int64
	Width       int
	Height      int
	Frames      int
}
