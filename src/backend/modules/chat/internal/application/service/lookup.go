package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
)

// Lookup finds the newest state of a message of a tenant (F118): the recent buffer holds the newest revisions that
// chat-persister may not have applied yet, PostgreSQL the persisted state. The newest revision wins; a recall seen in
// either wins. Display content only from PostgreSQL; the recent copy may carry the original body (evidence for the
// revision of an edit, never served to a user).
type Lookup struct {
	Read   ReadRunner
	Reader read.FeatureReader
	Recent port.RecentBuffer
	Index  port.MessageIndex
}

// Found is a message found by Lookup.
type Found struct {
	Envelope message.Envelope
	// Persisted reports a row in PostgreSQL (reactions and pins wait for it before they are applied).
	Persisted bool
}

// Message returns the message id of tenant (nil: unknown in the tenant). A message of another tenant is never returned:
// the index and the recent buffer are checked against the tenant, PostgreSQL filters it (R8).
func (l Lookup) Message(ctx context.Context, tenant, id uuid.UUID) (*Found, error) {
	var channel uuid.UUID
	if l.Index != nil {
		t, ch, ok, err := l.Index.Get(ctx, id)
		if err != nil {
			return nil, unavailable(err)
		}
		if ok && t == tenant {
			channel = ch
		}
	}
	var stored *read.StoredMessage
	err := l.readTenant(ctx, tenant, func(ctx context.Context) error {
		var err error
		stored, err = l.Reader.MessageByID(ctx, tenant, id)
		return err
	})
	if err != nil {
		return nil, unavailable(err)
	}
	if stored != nil {
		channel = stored.Envelope.ChannelID
	}
	var best *Found
	if stored != nil {
		e := stored.Envelope
		best = &Found{Envelope: e, Persisted: true}
	}
	if channel != uuid.Nil {
		recent, err := l.Recent.List(ctx, channel)
		if err == nil {
			for _, e := range recent {
				if e.MessageID != id || e.TenantID != tenant {
					continue
				}
				switch {
				case best == nil:
					best = &Found{Envelope: e}
				case e.Revision > best.Envelope.Revision:
					persisted := best.Persisted
					if best.Envelope.Recalled() && !e.Recalled() {
						e.RecalledAt, e.RecalledBy, e.BodyDisplay, e.BodyOriginal = best.Envelope.RecalledAt, best.Envelope.RecalledBy, nil, nil
					}
					if best.Envelope.Moderation.State == message.StateRemoved {
						e.Moderation.State = message.StateRemoved
					}
					best = &Found{Envelope: e, Persisted: persisted}
				case e.Revision == best.Envelope.Revision && best.Persisted && best.Envelope.BodyOriginal == nil:
					best.Envelope.BodyOriginal = e.BodyOriginal // the original of the current version (evidence of an edit)
				}
			}
		}
	}
	return best, nil
}

func (l Lookup) readTenant(ctx context.Context, tenant uuid.UUID, fn func(ctx context.Context) error) error {
	return Facts{Read: l.Read}.read(ctx, tenant, fn)
}

// Refs returns the newest state of the messages ids of one channel (reply previews and quotes of a history page).
func (l Lookup) Refs(ctx context.Context, tenant uuid.UUID, ch uuid.UUID, gameplay bool, ids []uuid.UUID, recent []message.Envelope) (map[uuid.UUID]message.Envelope, error) {
	out := map[uuid.UUID]message.Envelope{}
	if len(ids) == 0 {
		return out, nil
	}
	var stored map[uuid.UUID]read.StoredMessage
	err := l.readTenant(ctx, tenant, func(ctx context.Context) error {
		var err error
		stored, err = l.Reader.MessagesByID(ctx, tenant, ch, gameplay, ids)
		return err
	})
	if err != nil {
		return nil, unavailable(err)
	}
	for id, m := range stored {
		out[id] = m.Envelope
	}
	want := map[uuid.UUID]bool{}
	for _, id := range ids {
		want[id] = true
	}
	for _, e := range recent {
		if !want[e.MessageID] || e.TenantID != tenant || e.ChannelID != ch {
			continue
		}
		prev, ok := out[e.MessageID]
		if !ok || e.Revision > prev.Revision {
			if ok && prev.Recalled() {
				continue
			}
			out[e.MessageID] = e
		}
	}
	return out, nil
}

// Assets implements port.AssetChecker on the library read model (F118-AC-12, AC-13).
type Assets struct {
	Read   ReadRunner
	Reader read.FeatureReader
}

var _ port.AssetChecker = Assets{}

// Sticker implements port.AssetChecker.
func (a Assets) Sticker(ctx context.Context, tenant, student, sticker uuid.UUID) (ok bool, err error) {
	err = Facts{Read: a.Read}.read(ctx, tenant, func(ctx context.Context) error {
		ok, err = a.Reader.StickerAvailable(ctx, tenant, student, sticker)
		return err
	})
	return ok, err
}

// Gif implements port.AssetChecker (the library is platform-wide; read in the session of the sender's tenant).
func (a Assets) Gif(ctx context.Context, tenant, gif uuid.UUID) (found bool, err error) {
	err = Facts{Read: a.Read}.read(ctx, tenant, func(ctx context.Context) error {
		found, err = a.Reader.GifAvailable(ctx, gif)
		return err
	})
	return found, err
}
