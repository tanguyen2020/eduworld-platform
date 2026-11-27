package command

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/contentmodel"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/learning/internal/domain/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/audit"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/ids"
	pkgmedia "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/media"
	"github.com/tanguyen2020/eduworld-platform/src/backend/pkg/problem"
)

// IngestInput is a file produced on the server (a picture embedded in a Word import, F028-AC-13).
type IngestInput struct {
	Kind     media.Kind
	MIME     string
	Filename string
	AltText  string
	License  string
	Data     []byte
}

// IngestFile puts a server-side file into the media pipeline like an upload (F027-AC-01…03): the same file in the
// same scope returns the existing asset; otherwise the object goes to ew-media-quarantine, the asset moves to
// processing with the outbox event media.uploaded (virus scan, variants) and the audit media.uploaded. It runs in the
// transaction of ctx; the caller authorized the operation it belongs to.
func (h Media) IngestFile(ctx context.Context, o contentmodel.Owner, actor uuid.UUID, in IngestInput) (uuid.UUID, error) {
	sum := sha256.Sum256(in.Data)
	u := media.Upload{Kind: in.Kind, MIME: in.MIME, Size: int64(len(in.Data)), AltText: in.AltText, Owner: o, SHA256: hex.EncodeToString(sum[:]),
		Filename: in.Filename, License: in.License, UploadedBy: actor}
	if err := u.Validate(); err != nil {
		return uuid.Nil, uploadProblem(ctx, err)
	}
	dup, found, err := h.Store.AssetBySHA(ctx, o, u.SHA256)
	if err != nil {
		return uuid.Nil, err
	}
	if found {
		return dup.ID, nil
	}
	id := ids.New()
	a := media.New(id, u, pkgmedia.QuarantineKey(id.String()))
	if err := h.Storage.Put(ctx, pkgmedia.BucketQuarantine, a.QuarantineKey, bytes.NewReader(in.Data), a.Size, a.MIME, ""); err != nil {
		return uuid.Nil, problem.New(problem.Unavailable, "object storage unavailable").Retry().Wrap(err)
	}
	if err := h.Store.InsertAsset(ctx, a); err != nil {
		return uuid.Nil, err
	}
	// SaveAsset is optimistic: continue from the stored row (its version), like CompleteMediaUpload.
	a, err = h.Store.Asset(ctx, id)
	if err != nil {
		return uuid.Nil, err
	}
	processing, err := a.Complete(media.Uploaded{Exists: true, Size: a.Size, SHA256: a.SHA256}, now(h.Clock))
	if err != nil {
		return uuid.Nil, err
	}
	if err := h.Store.SaveAsset(ctx, a); err != nil {
		return uuid.Nil, err
	}
	if processing {
		err = h.publish(ctx, a, EventMediaUploaded, UploadedPayload{AssetID: a.ID, TenantID: a.Owner.TenantID, Kind: string(a.Kind),
			MIME: a.MIME, QuarantineKey: a.QuarantineKey, SizeBytes: a.Size, SHA256: a.SHA256})
	} else {
		err = h.rejected(ctx, a)
	}
	if err != nil {
		return uuid.Nil, err
	}
	if err := h.trash().recordFor(ctx, a.Owner.TenantID, audit.Event{Action: ActionMediaUploaded, TargetType: string(contentmodel.ResourceMedia),
		TargetID: a.ID.String(), Result: audit.ResultSuccess, Metadata: map[string]any{"kind": string(a.Kind), "status": string(a.Status),
			"size_bytes": a.Size, "source": "import"}}); err != nil {
		return uuid.Nil, err
	}
	return a.ID, nil
}
