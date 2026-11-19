package media

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strconv"

	"github.com/HugoSmits86/nativewebp"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

// ImageSizes are the longest-side sizes of the image variants (CONTENT_MODEL §7: 256 / 1024 / 2048 px).
var ImageSizes = []int{256, 1024, 2048}

// MaxImagePixels refuses decompression bombs before decoding (50 megapixels).
const MaxImagePixels = 50_000_000

// ProcessImage decodes a raster image (PNG, JPEG, GIF first frame, WebP), applies the EXIF orientation of a JPEG,
// and re-encodes it as WebP (lossless, pure Go) and PNG at every size of sizes (longest side, never upscaled). The
// outputs are encoded from pixels only, so no EXIF/GPS/XMP/ICC metadata survives (F027-AC-05, SC-001).
func ProcessImage(data []byte, mime string, sizes []int) (Result, error) {
	cfg, err := decodeConfig(data, mime)
	if err != nil {
		return Result{}, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > MaxImagePixels {
		return Result{}, fmt.Errorf("%w: %dx%d pixels", ErrDecode, cfg.Width, cfg.Height)
	}
	img, err := decode(data, mime)
	if err != nil {
		return Result{}, err
	}
	if mime == "image/jpeg" {
		img = orient(img, jpegOrientation(data))
	}
	b := img.Bounds()
	res := Result{Width: b.Dx(), Height: b.Dy()}
	for _, s := range sizes {
		scaled := scale(img, s)
		var pb, wb bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(&pb, scaled); err != nil {
			return Result{}, fmt.Errorf("%w: png: %w", ErrTranscode, err)
		}
		if err := nativewebp.Encode(&wb, scaled, nil); err != nil {
			return Result{}, fmt.Errorf("%w: webp: %w", ErrTranscode, err)
		}
		n := strconv.Itoa(s)
		res.Outputs = append(res.Outputs,
			Output{Variant: "webp_" + n, File: "webp_" + n + ".webp", MIME: "image/webp", Data: wb.Bytes(), Main: true},
			Output{Variant: "png_" + n, File: "png_" + n + ".png", MIME: "image/png", Data: pb.Bytes(), Main: true})
	}
	return res, nil
}

func decodeConfig(data []byte, mime string) (image.Config, error) {
	r := bytes.NewReader(data)
	var (
		cfg image.Config
		err error
	)
	switch mime {
	case "image/png":
		cfg, err = png.DecodeConfig(r)
	case "image/jpeg":
		cfg, err = jpeg.DecodeConfig(r)
	case "image/gif":
		cfg, err = gif.DecodeConfig(r)
	case "image/webp":
		cfg, err = webp.DecodeConfig(r)
	default:
		return cfg, fmt.Errorf("%w: unsupported image type %s", ErrDecode, mime)
	}
	if err != nil {
		return cfg, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	return cfg, nil
}

func decode(data []byte, mime string) (image.Image, error) {
	r := bytes.NewReader(data)
	var (
		img image.Image
		err error
	)
	switch mime {
	case "image/png":
		img, err = png.Decode(r)
	case "image/jpeg":
		img, err = jpeg.Decode(r)
	case "image/gif":
		img, err = gif.Decode(r)
	case "image/webp":
		img, err = webp.Decode(r)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecode, err)
	}
	if img == nil {
		return nil, ErrDecode
	}
	return img, nil
}

// scale returns img with its longest side at most size (never upscaled), as NRGBA.
func scale(img image.Image, size int) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if m := max(w, h); m > size {
		w, h = max(1, w*size/m), max(1, h*size/m)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w == b.Dx() && h == b.Dy() {
		draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
		return dst
	}
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

// jpegOrientation reads the EXIF Orientation tag (0x0112) of a JPEG; 1 when absent or malformed.
func jpegOrientation(data []byte) int {
	i := 2
	for i+4 <= len(data) {
		if data[i] != 0xFF {
			return 1
		}
		marker := data[i+1]
		if marker == 0xDA || marker == 0xD9 { // start of scan / end of image
			return 1
		}
		segLen := int(binary.BigEndian.Uint16(data[i+2 : i+4]))
		end := i + 2 + segLen
		if segLen < 2 || end > len(data) {
			return 1
		}
		if marker == 0xE1 && segLen > 8 && bytes.HasPrefix(data[i+4:end], []byte("Exif\x00\x00")) {
			return tiffOrientation(data[i+10 : end])
		}
		i = end
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:8]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off : off+2]))
	for k := 0; k < n; k++ {
		e := off + 2 + 12*k
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:e+2]) == 0x0112 {
			v := int(bo.Uint16(t[e+8 : e+10]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// orient applies an EXIF orientation (1–8) so the pixels are upright once the metadata is gone.
func orient(img image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewNRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy int
			switch o {
			case 2:
				dx, dy = w-1-x, y
			case 3:
				dx, dy = w-1-x, h-1-y
			case 4:
				dx, dy = x, h-1-y
			case 5:
				dx, dy = y, x
			case 6:
				dx, dy = h-1-y, x
			case 7:
				dx, dy = h-1-y, w-1-x
			case 8:
				dx, dy = y, w-1-x
			}
			dst.Set(dx, dy, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
