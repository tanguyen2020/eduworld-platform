// Package mediaproc adapts the media toolchain of src/backend/pkg/media (clamd, image variants, ffmpeg) to the port
// MediaProcessor of module learning (F027 T017).
package mediaproc

import (
	"context"
	"io"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
)

// Processor implements port.MediaProcessor.
type Processor struct {
	Scanner interface {
		Scan(ctx context.Context, r io.Reader) (pkgmedia.Verdict, error)
	}
	FFmpeg pkgmedia.FFmpeg
}

var _ port.MediaProcessor = Processor{}

// New returns the processor of a worker: clamd at clamAddr (EW_CLAMAV_ADDR), ffmpeg/ffprobe on PATH.
func New(clamAddr string) Processor {
	return Processor{Scanner: pkgmedia.Clamd{Addr: clamAddr}}
}

// Scan implements port.MediaProcessor.
func (p Processor) Scan(ctx context.Context, r io.Reader) (pkgmedia.Verdict, error) {
	return p.Scanner.Scan(ctx, r)
}

// Image implements port.MediaProcessor (WebP + PNG, 256/1024/2048 px, no metadata).
func (p Processor) Image(data []byte, mime string) (pkgmedia.Result, error) {
	return pkgmedia.ProcessImage(data, mime, pkgmedia.ImageSizes)
}

// AudioVideo implements port.MediaProcessor.
func (p Processor) AudioVideo(ctx context.Context, video bool, input, workDir string, maxDuration time.Duration) (pkgmedia.Result, error) {
	jobs := pkgmedia.AudioJobs()
	if video {
		jobs = pkgmedia.VideoJobs()
	}
	return p.FFmpeg.Transcode(ctx, input, workDir, jobs, maxDuration)
}
