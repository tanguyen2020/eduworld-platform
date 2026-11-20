package media

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// FFmpeg transcodes audio and video with the ffmpeg/ffprobe binaries of the worker image (deploy/docker/
// worker.Dockerfile): audio → Ogg Vorbis (Godot) + MP3 (web); video → Ogg Theora .ogv (Godot 4) + MP4 H.264 720p
// (CMS/web) + HLS 360p/720p (CONTENT_MODEL §7). Metadata is dropped (-map_metadata -1). Each run has a timeout.
type FFmpeg struct {
	Bin     string        // ffmpeg (default "ffmpeg" on PATH)
	Probe   string        // ffprobe (default "ffprobe")
	Timeout time.Duration // per file (default 15 minutes)
}

// Available reports whether ffmpeg and ffprobe can be found.
func (f FFmpeg) Available() bool {
	_, e1 := exec.LookPath(f.bin())
	_, e2 := exec.LookPath(f.probe())
	return e1 == nil && e2 == nil
}

func (f FFmpeg) bin() string {
	if f.Bin != "" {
		return f.Bin
	}
	return "ffmpeg"
}

func (f FFmpeg) probe() string {
	if f.Probe != "" {
		return f.Probe
	}
	return "ffprobe"
}

// Job is one ffmpeg invocation producing one variant.
type Job struct {
	Variant string
	Main    string // main output file, relative to the work directory
	MIME    string
	Args    []string // arguments after "-i <input>"
}

// AudioJobs are the audio variants.
func AudioJobs() []Job {
	return []Job{
		{Variant: "ogg", Main: "ogg.ogg", MIME: "audio/ogg", Args: []string{"-vn", "-map_metadata", "-1", "-c:a", "libvorbis", "-q:a", "4", "ogg.ogg"}},
		{Variant: "mp3", Main: "mp3.mp3", MIME: "audio/mpeg", Args: []string{"-vn", "-map_metadata", "-1", "-c:a", "libmp3lame", "-q:a", "4", "mp3.mp3"}},
	}
}

func scaleTo(h int) string { return "scale=-2:'min(" + strconv.Itoa(h) + ",ih)'" }

// VideoJobs are the video variants: Theora 480p/720p for the game client (Low/Web and Medium/High presets, F065
// research R3), MP4 and HLS for the web.
func VideoJobs() []Job {
	hls := func(h int) Job {
		dir := "hls_" + strconv.Itoa(h)
		return Job{Variant: dir, Main: dir + "/index.m3u8", MIME: "application/vnd.apple.mpegurl", Args: []string{
			"-map_metadata", "-1", "-vf", scaleTo(h), "-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-c:a", "aac", "-b:a", "128k",
			"-f", "hls", "-hls_time", "6", "-hls_playlist_type", "vod", "-hls_segment_filename", dir + "/seg_%03d.ts", dir + "/index.m3u8"}}
	}
	return []Job{
		{Variant: "ogv_480", Main: "ogv_480.ogv", MIME: "video/ogg", Args: []string{"-map_metadata", "-1", "-vf", scaleTo(480),
			"-c:v", "libtheora", "-q:v", "5", "-c:a", "libvorbis", "-q:a", "3", "ogv_480.ogv"}},
		{Variant: "ogv_720", Main: "ogv_720.ogv", MIME: "video/ogg", Args: []string{"-map_metadata", "-1", "-vf", scaleTo(720),
			"-c:v", "libtheora", "-q:v", "6", "-c:a", "libvorbis", "-q:a", "4", "ogv_720.ogv"}},
		{Variant: "mp4_720", Main: "mp4_720.mp4", MIME: "video/mp4", Args: []string{"-map_metadata", "-1", "-vf", scaleTo(720),
			"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", "mp4_720.mp4"}},
		hls(360), hls(720),
	}
}

// ProbeFile returns the duration (ms) and the size of the first video stream of a file.
func (f FFmpeg) ProbeFile(ctx context.Context, path string) (durationMS, width, height int, err error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	out, err := exec.CommandContext(ctx, f.probe(), "-v", "error", "-print_format", "json", "-show_format", "-show_streams", path).Output() //nolint:gosec // fixed binary, path from the worker
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return 0, 0, 0, ErrToolMissing
		}
		return 0, 0, 0, fmt.Errorf("%w: ffprobe: %w", ErrDecode, err)
	}
	var p struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &p); err != nil {
		return 0, 0, 0, fmt.Errorf("%w: ffprobe output: %w", ErrDecode, err)
	}
	d, _ := strconv.ParseFloat(p.Format.Duration, 64)
	for _, s := range p.Streams {
		if s.CodecType == "video" {
			width, height = s.Width, s.Height
			break
		}
	}
	return int(math.Round(d * 1000)), width, height, nil
}

// Transcode runs jobs on input inside workDir (created by the caller, removed by it after upload). maxDuration
// > 0 refuses longer files with ErrTooLong before any transcoding.
func (f FFmpeg) Transcode(ctx context.Context, input, workDir string, jobs []Job, maxDuration time.Duration) (Result, error) {
	if !f.Available() {
		return Result{}, ErrToolMissing
	}
	d, w, h, err := f.ProbeFile(ctx, input)
	if err != nil {
		return Result{}, err
	}
	if maxDuration > 0 && time.Duration(d)*time.Millisecond > maxDuration {
		return Result{}, fmt.Errorf("%w: %d ms", ErrTooLong, d)
	}
	timeout := f.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Minute
	}
	res := Result{Width: w, Height: h, DurationMS: d}
	for _, j := range jobs {
		if dir := filepath.Dir(filepath.Join(workDir, j.Main)); dir != workDir {
			if err := os.MkdirAll(dir, 0o750); err != nil {
				return Result{}, err
			}
		}
		jctx, cancel := context.WithTimeout(ctx, timeout)
		args := append([]string{"-nostdin", "-y", "-loglevel", "error", "-i", input}, j.Args...)
		cmd := exec.CommandContext(jctx, f.bin(), args...) //nolint:gosec // fixed binary and arguments
		cmd.Dir = workDir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		err := cmd.Run()
		cancel()
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if len(msg) > 300 {
				msg = msg[:300]
			}
			return Result{}, fmt.Errorf("%w: %s: %w %s", ErrTranscode, j.Variant, err, msg)
		}
		outs, err := collect(workDir, j)
		if err != nil {
			return Result{}, err
		}
		res.Outputs = append(res.Outputs, outs...)
	}
	return res, nil
}

// collect lists the files of a job: the main file, plus every file of its directory for multi-file variants.
func collect(workDir string, j Job) ([]Output, error) {
	var out []Output
	if !strings.Contains(j.Main, "/") {
		st, err := os.Stat(filepath.Join(workDir, j.Main))
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", ErrTranscode, j.Variant, err)
		}
		return []Output{{Variant: j.Variant, File: j.Main, MIME: j.MIME, Path: filepath.Join(workDir, j.Main), Bytes: st.Size(), Main: true}}, nil
	}
	dir := filepath.Join(workDir, filepath.Dir(j.Main))
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		st, err := d.Info()
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(workDir, p)
		rel = filepath.ToSlash(rel)
		mime := "video/mp2t"
		if strings.HasSuffix(rel, ".m3u8") {
			mime = "application/vnd.apple.mpegurl"
		}
		out = append(out, Output{Variant: j.Variant, File: rel, MIME: mime, Path: p, Bytes: st.Size(), Main: rel == j.Main})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", ErrTranscode, j.Variant, err)
	}
	return out, nil
}
