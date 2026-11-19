// Package mediatest holds TEST FIXTURES (R17) of the media pipeline: a JPEG carrying EXIF orientation and GPS data,
// as a phone photo does, and a silent WAV. Never used by a binary.
package mediatest

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// JPEGWithEXIF returns a w×h JPEG with an EXIF APP1 segment: Orientation = orientation and a GPS IFD.
func JPEGWithEXIF(t testing.TB, w, h, orientation int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 90, A: 255}) //nolint:gosec // test pattern
		}
	}
	var raw bytes.Buffer
	if err := jpeg.Encode(&raw, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	le := binary.LittleEndian
	tiff := []byte("II*\x00\x08\x00\x00\x00")
	ifd := make([]byte, 2+2*12+4)
	le.PutUint16(ifd[0:], 2)
	le.PutUint16(ifd[2:], 0x0112) // Orientation, SHORT
	le.PutUint16(ifd[4:], 3)
	le.PutUint32(ifd[6:], 1)
	le.PutUint16(ifd[10:], uint16(orientation)) //nolint:gosec // 1–8
	le.PutUint16(ifd[14:], 0x8825)              // GPSInfo IFD pointer
	le.PutUint16(ifd[16:], 4)
	le.PutUint32(ifd[18:], 1)
	le.PutUint32(ifd[22:], uint32(8+len(ifd))) //nolint:gosec // small
	gps := make([]byte, 2+12+4)
	le.PutUint16(gps[0:], 1)
	le.PutUint16(gps[2:], 0x0001) // GPSLatitudeRef "N"
	le.PutUint16(gps[4:], 2)
	le.PutUint32(gps[6:], 2)
	copy(gps[10:], "N\x00")
	exif := append([]byte("Exif\x00\x00"), tiff...)
	exif = append(append(exif, ifd...), gps...)
	exif = append(exif, []byte("GPSLatitude=21.0285")...)
	seg := []byte{0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(exif)+2)) //nolint:gosec // small
	out := append([]byte{0xFF, 0xD8}, seg...)
	out = append(out, exif...)
	return append(out, raw.Bytes()[2:]...)
}

// SilentWAV is one second of 8-bit mono silence at rate Hz.
func SilentWAV(rate int) []byte {
	var b bytes.Buffer
	le := binary.LittleEndian
	u32 := func(v int) { _ = binary.Write(&b, le, uint32(v)) } //nolint:gosec // small
	u16 := func(v int) { _ = binary.Write(&b, le, uint16(v)) } //nolint:gosec // small
	data := bytes.Repeat([]byte{128}, rate)
	b.WriteString("RIFF")
	u32(36 + len(data))
	b.WriteString("WAVEfmt ")
	u32(16)
	u16(1)
	u16(1)
	u32(rate)
	u32(rate)
	u16(1)
	u16(8)
	b.WriteString("data")
	u32(len(data))
	b.Write(data)
	return b.Bytes()
}
