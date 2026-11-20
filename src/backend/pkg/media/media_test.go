package media

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"image"
	"image/png"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/webp"

	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media/mediatest"
)

func TestSniffAndCompatible(t *testing.T) {
	cases := []struct {
		head     []byte
		detected string
		declared string
		ok       bool
	}{
		{[]byte("\x89PNG\r\n\x1a\nrest"), "image/png", "image/png", true},
		{[]byte{0xFF, 0xD8, 0xFF, 0xE0}, "image/jpeg", "image/jpg", true},
		{[]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "image/webp", "image/webp", true},
		{[]byte("RIFF\x00\x00\x00\x00WAVEfmt "), "audio/wav", "audio/x-wav", true},
		{[]byte("ID3\x03\x00"), "audio/mpeg", "audio/mpeg", true},
		{[]byte("OggS\x00\x02"), "audio/ogg", "audio/ogg", true},
		{[]byte("\x00\x00\x00\x18ftypmp42"), "video/mp4", "video/mp4", true},
		{[]byte{0x1A, 0x45, 0xDF, 0xA3}, "video/webm", "video/webm", true},
		{[]byte("glTF\x02\x00"), "model/gltf-binary", "model/gltf-binary", true},
		{[]byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"/>`), "image/svg+xml", "image/svg+xml", true},
		{[]byte("a,b,c\n1,2,3\n"), "text/plain", "text/csv", true},
		// F027 follow-up (F065 OQ): WebVTT subtitles, with or without BOM; text that is not WebVTT declared as text/vtt
		// (an SRT, a header without its separator) is a type mismatch, and a VTT declared as CSV too.
		{[]byte("WEBVTT\n\n00:00.000 --> 00:04.000\nXin chào!\n"), "text/vtt", "text/vtt", true},
		{[]byte("\xef\xbb\xbfWEBVTT - phụ đề\r\n"), "text/vtt", "text/vtt", true},
		{[]byte("WEBVTT"), "text/vtt", "text/vtt", true},
		{[]byte("WEBVTTX\n"), "text/plain", "text/vtt", false},
		{[]byte("1\n00:00:00,000 --> 00:00:04,000\nSRT\n"), "text/plain", "text/vtt", false},
		{[]byte("WEBVTT\n"), "text/vtt", "text/csv", false},
		{[]byte("PK\x03\x04"), "application/zip", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", true},
		// A PDF renamed .png and the EICAR text declared as PNG are type mismatches (quickstart step 5).
		{[]byte("%PDF-1.7"), "application/pdf", "image/png", false},
		{[]byte(`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR`), "text/plain", "image/png", false},
	}
	for _, c := range cases {
		if got := Sniff(c.head); got != c.detected {
			t.Errorf("Sniff(%q) = %q, want %q", c.head, got, c.detected)
		}
		if got := Compatible(c.declared, c.detected); got != c.ok {
			t.Errorf("Compatible(%q, %q) = %v", c.declared, c.detected, got)
		}
	}
}

// SC-001: no variant keeps EXIF/GPS; the EXIF orientation is applied before the metadata is dropped.
func TestProcessImageStripsMetadata(t *testing.T) {
	src := mediatest.JPEGWithEXIF(t, 300, 200, 6)
	if !bytes.Contains(src, []byte("GPSLatitude")) || jpegOrientation(src) != 6 {
		t.Fatal("fixture has no EXIF")
	}
	res, err := ProcessImage(src, "image/jpeg", ImageSizes)
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 200 || res.Height != 300 {
		t.Errorf("orientation 6 not applied: %dx%d", res.Width, res.Height)
	}
	if len(res.Outputs) != 6 {
		t.Fatalf("outputs = %d", len(res.Outputs))
	}
	for _, o := range res.Outputs {
		if bytes.Contains(o.Data, []byte("Exif")) || bytes.Contains(o.Data, []byte("GPS")) {
			t.Errorf("%s keeps metadata", o.File)
		}
		var cfg image.Config
		var err error
		switch o.MIME {
		case "image/png":
			cfg, err = png.DecodeConfig(bytes.NewReader(o.Data))
		case "image/webp":
			cfg, err = webp.DecodeConfig(bytes.NewReader(o.Data))
		}
		if err != nil {
			t.Fatalf("%s: %v", o.File, err)
		}
		if strings.HasSuffix(o.Variant, "_256") && max(cfg.Width, cfg.Height) != 256 {
			t.Errorf("%s: %dx%d", o.File, cfg.Width, cfg.Height)
		}
		if strings.HasSuffix(o.Variant, "_1024") && (cfg.Width != 200 || cfg.Height != 300) {
			t.Errorf("%s upscaled: %dx%d", o.File, cfg.Width, cfg.Height)
		}
	}
}

func TestProcessImageRejectsCorrupt(t *testing.T) {
	bad := append([]byte("\x89PNG\r\n\x1a\n"), bytes.Repeat([]byte{1}, 64)...)
	if _, err := ProcessImage(bad, "image/png", ImageSizes); !errors.Is(err, ErrDecode) {
		t.Fatalf("err = %v", err)
	}
}

func TestKeys(t *testing.T) {
	sha := strings.Repeat("a", 64)
	if VariantKey(sha, "webp_256.webp") != "media/"+sha+"/webp_256.webp" || MathKey(sha) != "math/"+sha+".svg" ||
		QuarantineKey("x") != "quarantine/x" {
		t.Fatal("keys")
	}
}

// fakeClamd is a TEST DOUBLE (R17) of clamd INSTREAM: infected when the stream contains "EICAR".
func fakeClamd(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer func() { _ = c.Close() }()
				cmd := make([]byte, 10)
				if _, err := io.ReadFull(c, cmd); err != nil {
					return
				}
				var data []byte
				for {
					var n uint32
					if err := binary.Read(c, binary.BigEndian, &n); err != nil || n == 0 {
						break
					}
					b := make([]byte, n)
					if _, err := io.ReadFull(c, b); err != nil {
						return
					}
					data = append(data, b...)
				}
				reply := "stream: OK\x00"
				if bytes.Contains(data, []byte("EICAR")) {
					reply = "stream: Eicar-Test-Signature FOUND\x00"
				}
				_, _ = c.Write([]byte(reply))
			}(c)
		}
	}()
	return ln.Addr().String()
}

func TestClamd(t *testing.T) {
	ctx := context.Background()
	c := Clamd{Addr: fakeClamd(t)}
	v, err := c.Scan(ctx, strings.NewReader("hello"))
	if err != nil || !v.Clean {
		t.Fatalf("clean: %+v %v", v, err)
	}
	v, err = c.Scan(ctx, strings.NewReader(`X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`))
	if err != nil || v.Clean || v.Signature != "Eicar-Test-Signature" {
		t.Fatalf("infected: %+v %v", v, err)
	}
	if _, err := (Clamd{Addr: "127.0.0.1:1"}).Scan(ctx, strings.NewReader("x")); !errors.Is(err, ErrScannerUnavailable) {
		t.Fatalf("unavailable: %v", err)
	}
}

func TestFFmpegJobs(t *testing.T) {
	for _, j := range append(AudioJobs(), VideoJobs()...) {
		joined := strings.Join(j.Args, " ")
		if !strings.Contains(joined, "-map_metadata -1") || !strings.Contains(joined, j.Main) {
			t.Errorf("%s: %s", j.Variant, joined)
		}
	}
	f := FFmpeg{Bin: "ffmpeg-missing-for-test", Probe: "ffprobe-missing-for-test"}
	if _, err := f.Transcode(context.Background(), "in", t.TempDir(), AudioJobs(), 0); !errors.Is(err, ErrToolMissing) {
		t.Fatalf("err = %v", err)
	}
}

// TestFFmpegAudio runs the real ffmpeg when it is installed (worker image, CI); skipped elsewhere.
func TestFFmpegAudio(t *testing.T) {
	f := FFmpeg{}
	if !f.Available() {
		t.Skip("ffmpeg/ffprobe not on PATH")
	}
	dir := t.TempDir()
	in := filepath.Join(dir, "in.wav")
	if err := os.WriteFile(in, mediatest.SilentWAV(8000), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := f.Transcode(context.Background(), in, dir, AudioJobs(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Outputs) != 2 || res.DurationMS < 900 {
		t.Fatalf("result %+v", res)
	}
}
