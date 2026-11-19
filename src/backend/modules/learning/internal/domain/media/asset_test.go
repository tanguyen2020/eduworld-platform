package media

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
)

var (
	tenant   = uuid.New()
	platform = contentmodel.Owner{Scope: contentmodel.OwnerPlatform}
	ownTen   = contentmodel.Owner{Scope: contentmodel.OwnerTenant, TenantID: &tenant}
	sha      = strings.Repeat("a", 64)
)

// F027-AC-01, AC-02: allowed types, size limits, alt text of images, 3D models platform only.
func TestUploadValidate(t *testing.T) {
	ok := Upload{Kind: KindImage, MIME: "image/jpeg", Size: 1 << 20, AltText: "con mèo", Owner: ownTen}
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(u *Upload)
		want error
	}{
		{"exe", func(u *Upload) { u.MIME = "application/x-msdownload" }, ErrTypeNotAllowed},
		{"video as image", func(u *Upload) { u.MIME = "video/mp4" }, ErrTypeNotAllowed},
		{"11 MB image", func(u *Upload) { u.Size = 11 << 20 }, ErrTooLarge},
		{"empty", func(u *Upload) { u.Size = 0 }, ErrTooLarge},
		{"no alt", func(u *Upload) { u.AltText = "  " }, ErrAltTextRequired},
		{"unknown kind", func(u *Upload) { u.Kind = "font" }, ErrUnknownKind},
		{"tenant 3D", func(u *Upload) { u.Kind, u.MIME = KindModel3D, "model/gltf-binary" }, ErrPlatformOnly},
	}
	for _, c := range cases {
		u := ok
		c.edit(&u)
		if err := u.Validate(); !errors.Is(err, c.want) {
			t.Errorf("%s: %v, want %v", c.name, err, c.want)
		}
	}
	for _, u := range []Upload{
		{Kind: KindAudio, MIME: "audio/mpeg; charset=binary", Size: 20 << 20, Owner: ownTen},
		{Kind: KindVideo, MIME: "video/webm", Size: 300 << 20, Owner: ownTen},
		{Kind: KindModel3D, MIME: "model/gltf-binary", Size: 50 << 20, Owner: platform},
		{Kind: KindDocument, MIME: "text/csv", Size: 10 << 20, Owner: ownTen},
	} {
		if err := u.Validate(); err != nil {
			t.Errorf("%s %s: %v", u.Kind, u.MIME, err)
		}
	}
	if (Upload{Kind: KindAudio, MIME: "audio/mpeg", Size: 20<<20 + 1, Owner: ownTen}).Validate() == nil {
		t.Error("21 MB audio accepted")
	}
}

// F027-AC-06: platform content in the public bucket; tenant content, exam material and documents private.
func TestBucketFor(t *testing.T) {
	if BucketFor(platform, false, KindImage) != BucketPublic || BucketFor(platform, true, KindImage) != BucketPrivate ||
		BucketFor(ownTen, false, KindImage) != BucketPrivate || BucketFor(platform, false, KindDocument) != BucketPrivate {
		t.Fatal("buckets")
	}
}

// F027-AC-03, AC-04, AC-05: the pipeline state machine.
func TestAssetStateMachine(t *testing.T) {
	at := time.Now()
	newAsset := func() Asset {
		return New(uuid.New(), Upload{Kind: KindImage, MIME: "IMAGE/PNG", Size: 100, AltText: " a ", Owner: ownTen, SHA256: strings.ToUpper(sha)}, "quarantine/x")
	}
	a := newAsset()
	if a.Status != StatusQuarantined || a.Bucket != BucketPrivate || a.MIME != "image/png" || *a.AltText != "a" || a.SHA256 != sha {
		t.Fatalf("new: %+v", a)
	}
	if ok, err := a.Complete(Uploaded{Exists: true, Size: 100, SHA256: sha}, at); !ok || err != nil || a.Status != StatusProcessing {
		t.Fatalf("complete: %v %v %s", ok, err, a.Status)
	}
	if _, err := a.Complete(Uploaded{Size: 100, SHA256: sha}, at); !errors.Is(err, ErrNotQuarantined) {
		t.Fatalf("complete twice: %v", err)
	}
	if err := a.MarkReady(map[string]Variant{"webp_256": {Key: "media/x/webp_256.webp", Bytes: 1}}, 10, 20, 0, at); err != nil ||
		a.Status != StatusReady || *a.Width != 10 || a.DurationMS != nil {
		t.Fatalf("ready: %v %+v", err, a)
	}
	if err := a.Reject(RejectVirus, at); !errors.Is(err, ErrNotProcessing) {
		t.Fatalf("reject ready: %v", err)
	}
	b := newAsset()
	if ok, _ := b.Complete(Uploaded{Size: 100, SHA256: strings.Repeat("b", 64)}, at); ok || b.Status != StatusRejected || *b.RejectReason != "type_mismatch" {
		t.Fatalf("sha mismatch: %+v", b)
	}
	c := newAsset()
	if ok, _ := c.Complete(Uploaded{Size: 101, SHA256: sha}, at); ok || *c.RejectReason != "too_large" {
		t.Fatalf("larger: %+v", c)
	}
	d := newAsset()
	_, _ = d.Complete(Uploaded{Size: 100, SHA256: sha}, at)
	if err := d.Reject(RejectTranscodeFailed, at); err != nil || d.Status != StatusRejected || d.ProcessedAt == nil {
		t.Fatalf("reject: %v %+v", err, d)
	}
}
