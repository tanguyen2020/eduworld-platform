package media

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// SniffLen is the number of leading bytes Sniff needs.
const SniffLen = 512

// Sniff detects the type of a file from its first bytes (magic numbers; research F027 R1): images, audio, video,
// glTF binary, Godot packs, PDF, ZIP containers (xlsx), SVG, WebVTT and UTF-8 text. It returns "" when nothing matches.
func Sniff(head []byte) string {
	h := head
	if len(h) > SniffLen {
		h = h[:SniffLen]
	}
	switch {
	case bytes.HasPrefix(h, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	case bytes.HasPrefix(h, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(h, []byte("GIF87a")), bytes.HasPrefix(h, []byte("GIF89a")):
		return "image/gif"
	case len(h) >= 12 && bytes.Equal(h[:4], []byte("RIFF")) && bytes.Equal(h[8:12], []byte("WEBP")):
		return "image/webp"
	case len(h) >= 12 && bytes.Equal(h[:4], []byte("RIFF")) && bytes.Equal(h[8:12], []byte("WAVE")):
		return "audio/wav"
	case bytes.HasPrefix(h, []byte("OggS")):
		return "audio/ogg"
	case bytes.HasPrefix(h, []byte("ID3")), len(h) >= 2 && h[0] == 0xFF && h[1]&0xE0 == 0xE0:
		return "audio/mpeg"
	case len(h) >= 12 && bytes.Equal(h[4:8], []byte("ftyp")):
		return "video/mp4"
	case bytes.HasPrefix(h, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "video/webm"
	case bytes.HasPrefix(h, []byte("glTF")):
		return "model/gltf-binary"
	case bytes.HasPrefix(h, []byte("GDPC")):
		return "application/x-godot-pck"
	case bytes.HasPrefix(h, []byte("%PDF-")):
		return "application/pdf"
	case bytes.HasPrefix(h, []byte("PK\x03\x04")):
		return "application/zip"
	case isSVG(h):
		return "image/svg+xml"
	case isWebVTT(h):
		return "text/vtt"
	case isText(h):
		return "text/plain"
	}
	return ""
}

const bom = "\xef\xbb\xbf"

func isSVG(h []byte) bool {
	s := strings.TrimLeft(strings.TrimPrefix(string(h), bom), " \t\r\n")
	if !strings.HasPrefix(s, "<") || !utf8.ValidString(strings.ToValidUTF8(s, "")) {
		return false
	}
	return strings.Contains(strings.ToLower(s), "<svg")
}

// isWebVTT reports a UTF-8 text starting with the WebVTT signature: optional BOM, "WEBVTT", then end of file, a
// space, a tab or a line break (W3C WebVTT §4.1).
func isWebVTT(h []byte) bool {
	s := bytes.TrimPrefix(h, []byte(bom))
	if !bytes.HasPrefix(s, []byte("WEBVTT")) || !isText(h) {
		return false
	}
	rest := s[len("WEBVTT"):]
	return len(rest) == 0 || strings.IndexByte(" \t\r\n", rest[0]) >= 0
}

func isText(h []byte) bool {
	if len(h) == 0 || bytes.IndexByte(h, 0) >= 0 {
		return false
	}
	// The window may cut a multi-byte rune at its end.
	for i := 0; i < 3 && !utf8.Valid(h) && len(h) > 0; i++ {
		h = h[:len(h)-1]
	}
	return utf8.Valid(h)
}

// mimeFamily groups the declared types a detected type is compatible with.
var mimeFamily = map[string][]string{
	"image/png":                {"image/png"},
	"image/jpeg":               {"image/jpeg", "image/jpg", "image/pjpeg"},
	"image/gif":                {"image/gif"},
	"image/webp":               {"image/webp"},
	"image/svg+xml":            {"image/svg+xml"},
	"audio/wav":                {"audio/wav", "audio/x-wav", "audio/wave", "audio/vnd.wave"},
	"audio/ogg":                {"audio/ogg", "video/ogg", "application/ogg", "audio/vorbis"},
	"audio/mpeg":               {"audio/mpeg", "audio/mp3", "audio/mpeg3"},
	"video/mp4":                {"video/mp4", "audio/mp4", "audio/x-m4a", "video/quicktime"},
	"video/webm":               {"video/webm", "audio/webm", "video/x-matroska"},
	"model/gltf-binary":        {"model/gltf-binary"},
	"application/x-godot-pck":  {"application/x-godot-pck", "application/octet-stream"},
	"application/pdf":          {"application/pdf"},
	"application/zip":          {"application/zip", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	"text/vtt":                 {"text/vtt"},
	"text/plain":               {"text/plain", "text/csv", "application/json", "application/x-yaml", "application/yaml", "text/yaml", "application/xml", "text/xml"},
	"application/octet-stream": {"application/octet-stream"},
}

// Compatible reports whether the declared MIME type matches the detected one (magic bytes check, F027-AC-04).
func Compatible(declared, detected string) bool {
	declared = strings.ToLower(strings.TrimSpace(strings.SplitN(declared, ";", 2)[0]))
	for _, m := range mimeFamily[detected] {
		if m == declared {
			return true
		}
	}
	return false
}
