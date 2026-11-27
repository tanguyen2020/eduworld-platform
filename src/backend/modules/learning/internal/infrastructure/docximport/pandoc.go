package docximport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/application/port"
	dom "github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/docximport"
)

// Code defaults of CONFIG_DEFAULTS (owner CONTENT_PRODUCTION_PLAN §8, scope code).
const (
	ConvertTimeout  = 60 * time.Second // docx_convert_timeout
	ConvertMemoryMB = 512              // docx_convert_memory
	MaxASTBytes     = 32 << 20         // JSON AST read from Pandoc (bounded stdout)
)

// Pandoc is the port.DocxConverter of the worker: Pandoc 3.6 of the image eduworld/media-tools (stage media-tools of
// deploy/docker/worker.Dockerfile) run as an external process. Command is the program and its leading arguments
// (default ["pandoc"]; tests use a docker run of the image); Env the environment of the process (nil = empty but
// LANG, so nothing of the worker leaks into it).
type Pandoc struct {
	Command  []string
	Env      []string
	Timeout  time.Duration
	MemoryMB int
	Limits   ZipLimits
}

var _ port.DocxConverter = Pandoc{}

func (p Pandoc) limits() ZipLimits {
	if p.Limits == (ZipLimits{}) {
		return DefaultZipLimits
	}
	return p.Limits
}

// Available reports whether the converter program can be found.
func (p Pandoc) Available() bool {
	_, err := exec.LookPath(p.command()[0])
	return err == nil
}

func (p Pandoc) command() []string {
	if len(p.Command) == 0 {
		return []string{"pandoc"}
	}
	return p.Command
}

// Convert checks the zip, runs `pandoc --sandbox -f docx -t json` on it (stdin → stdout) and converts the AST.
func (p Pandoc) Convert(ctx context.Context, data []byte) (dom.Document, error) {
	ins, err := inspect(data, p.limits())
	if err != nil {
		return dom.Document{}, err
	}
	out, err := p.run(ctx, data)
	if err != nil {
		return dom.Document{}, err
	}
	return toDocument(out, ins.olePreviews)
}

// Media reads an embedded picture of the .docx (≤ the image media limit).
func (p Pandoc) Media(data []byte, path string) ([]byte, error) {
	return media(data, path, p.limits().MaxMedia)
}

// limitedBuffer keeps at most max bytes and records an overflow.
type limitedBuffer struct {
	buf      bytes.Buffer
	max      int
	overflow bool
}

func (l *limitedBuffer) Write(b []byte) (int, error) {
	if l.buf.Len()+len(b) > l.max {
		l.overflow = true
		return 0, errors.New("docximport: output limit reached")
	}
	return l.buf.Write(b)
}

func (p Pandoc) run(ctx context.Context, data []byte) ([]byte, error) {
	timeout := p.Timeout
	if timeout <= 0 {
		timeout = ConvertTimeout
	}
	mem := p.MemoryMB
	if mem <= 0 {
		mem = ConvertMemoryMB
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	dir, err := os.MkdirTemp("", "ew-docx-")
	if err != nil {
		return nil, fmt.Errorf("docximport: work dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	c := p.command()
	args := append(append([]string{}, c[1:]...), "--sandbox", "-f", "docx", "-t", "json", "+RTS", "-M"+strconv.Itoa(mem)+"m", "-RTS")
	cmd := exec.CommandContext(ctx, c[0], args...) //nolint:gosec // fixed program of the worker image, fixed arguments
	cmd.Dir = dir
	cmd.Env = p.Env
	if cmd.Env == nil {
		cmd.Env = []string{"LANG=C.UTF-8", "HOME=" + dir}
	}
	cmd.Stdin = bytes.NewReader(data)
	out := &limitedBuffer{max: MaxASTBytes}
	cmd.Stdout = out
	cmd.Stderr = io.Discard // Pandoc messages may quote the document: never kept or logged (R14)
	cmd.WaitDelay = 5 * time.Second
	err = cmd.Run()
	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return nil, port.ErrDocxTimeout
	case errors.Is(err, exec.ErrNotFound):
		return nil, port.ErrDocxNoConverter
	case out.overflow:
		return nil, port.ErrDocxTooComplex
	case err != nil:
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, port.ErrDocxInvalid
		}
		return nil, port.ErrDocxNoConverter
	}
	return out.buf.Bytes(), nil
}
