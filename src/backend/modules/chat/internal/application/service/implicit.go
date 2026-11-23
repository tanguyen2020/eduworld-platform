package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/channel"
)

// Implicit resolves the implicit channels of a student (F120-AC-12, AC-13): the World channel of its school (or of the
// tenant when a tenant unit set world_scope tenant, ADR-0054) and the Area channel of the hub instance the realtime
// gateway reported. Access is checked per message from the projections — never from channel_members — and is the same
// for every grade (R15). Read only.
type Implicit struct {
	Reader   read.ImplicitReader
	Policies read.PolicyReader
	Chat     read.ChatReader
	Rooms    port.RoomTracker // nil: no Area channel
	Cache    port.Cache
	Facts    Facts
}

func (s Implicit) student(ctx context.Context, tenant, user uuid.UUID) (*uuid.UUID, error) {
	raw, err := s.Cache.Get(ctx, "student:"+tenant.String()+":"+user.String(), s.Facts.Config.FactsFresh, func(ctx context.Context) ([]byte, error) {
		var school *uuid.UUID
		err := s.Facts.read(ctx, tenant, func(ctx context.Context) error {
			var err error
			school, err = s.Reader.StudentSchool(ctx, tenant, user)
			return err
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(school)
	})
	if err != nil {
		return nil, unavailable(err)
	}
	var school *uuid.UUID
	if err := json.Unmarshal(raw, &school); err != nil {
		return nil, unavailable(err)
	}
	return school, nil
}

// School returns the projected school of an active student (nil: unknown or not active).
func (s Implicit) School(ctx context.Context, tenant, user uuid.UUID) (*uuid.UUID, error) {
	return s.student(ctx, tenant, user)
}

// worldTenantWide reports world_scope tenant in force: a tenant unit whose tenant level asks for it.
func (s Implicit) worldTenantWide(ctx context.Context, tenant uuid.UUID) (bool, error) {
	key := "worldscope:" + tenant.String()
	if s.Facts.Gens != nil {
		if g, err := s.Facts.Gens.Generation(ctx, tenant); err == nil && g != "" {
			key += ":g" + g
		}
	}
	raw, err := s.Cache.Get(ctx, key, s.Facts.Config.PolicyCache, func(ctx context.Context) ([]byte, error) {
		wide := false
		err := s.Facts.read(ctx, tenant, func(ctx context.Context) error {
			typ, err := s.Policies.TenantType(ctx, tenant)
			if err != nil || typ != "unit" {
				return err
			}
			levels, err := s.Chat.PolicyLevels(ctx, tenant, nil)
			if err != nil {
				return err
			}
			wide = levels.WorldScope(true) == "tenant"
			return nil
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(wide)
	})
	if err != nil {
		return false, unavailable(err)
	}
	var wide bool
	if err := json.Unmarshal(raw, &wide); err != nil {
		return false, unavailable(err)
	}
	return wide, nil
}

// World returns the World channel id and scope of a student (ok false: not an active projected student).
func (s Implicit) World(ctx context.Context, tenant, user uuid.UUID) (id uuid.UUID, school *uuid.UUID, ok bool, err error) {
	sc, err := s.student(ctx, tenant, user)
	if err != nil || sc == nil {
		return uuid.Nil, nil, false, err
	}
	wide, err := s.worldTenantWide(ctx, tenant)
	if err != nil {
		return uuid.Nil, nil, false, err
	}
	if wide {
		return channel.WorldID(tenant, nil), nil, true, nil
	}
	return channel.WorldID(tenant, sc), sc, true, nil
}

// Area returns the Area channel of the hub instance of a student (ok false: no room reported by the gateway).
func (s Implicit) Area(ctx context.Context, tenant, user uuid.UUID) (id uuid.UUID, room string, ok bool, err error) {
	if s.Rooms == nil {
		return uuid.Nil, "", false, nil
	}
	room, err = s.Rooms.Room(ctx, tenant, user)
	if err != nil {
		return uuid.Nil, "", false, unavailable(err)
	}
	if room == "" || len(room) > 64 {
		return uuid.Nil, "", false, nil
	}
	return channel.AreaID(tenant, room), room, true, nil
}

// Access reports whether user may read/send in the implicit channel ch, with the school of the student (policy level).
// World: the channel of its school (or of its tenant when world_scope tenant is in force). Area: the room reported by
// the gateway equals the room of the channel (F120-AC-13).
func (s Implicit) Access(ctx context.Context, tenant, user uuid.UUID, ch channel.Channel) (bool, *uuid.UUID, error) {
	if ch.TenantID != tenant || !ch.Type.Implicit() {
		return false, nil, nil
	}
	school, err := s.student(ctx, tenant, user)
	if err != nil || school == nil {
		return false, nil, err
	}
	switch ch.Type {
	case channel.World:
		id, _, ok, err := s.World(ctx, tenant, user)
		if err != nil || !ok {
			return false, nil, err
		}
		return id == ch.ID, school, nil
	case channel.Area:
		id, _, ok, err := s.Area(ctx, tenant, user)
		if err != nil || !ok {
			return false, nil, err
		}
		return id == ch.ID, school, nil
	}
	return false, nil, nil
}

// Card returns the public card of a sender of an implicit channel (Level 1, no nickname when no profile).
func (s Implicit) Card(ctx context.Context, tenant, user uuid.UUID) read.ChatCard {
	card := read.ChatCard{UserID: user, Level: 1}
	raw, err := s.Cache.Get(ctx, "card:"+tenant.String()+":"+user.String(), s.Facts.Config.FactsFresh, func(ctx context.Context) ([]byte, error) {
		var c *read.ChatCard
		err := s.Facts.read(ctx, tenant, func(ctx context.Context) error {
			var err error
			c, err = s.Reader.Card(ctx, tenant, user)
			return err
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(c)
	})
	if err != nil {
		return card
	}
	var c *read.ChatCard
	if json.Unmarshal(raw, &c) == nil && c != nil {
		return *c
	}
	return card
}

func relationsKey(tenant, user uuid.UUID) string {
	return "rel:" + tenant.String() + ":" + user.String()
}

// Relations returns the live block and mute relations of a student (F120 T061: the World/Area fan-out leaves out the
// students it hides from, the history leaves out the senders it hides), cached chat_facts_cache like the blocks of the
// channel facts and dropped by InvalidateRelations when a relation changes. An error means the caller fails closed.
func (s Implicit) Relations(ctx context.Context, tenant, user uuid.UUID) (read.UserRelations, error) {
	raw, err := s.Cache.Get(ctx, relationsKey(tenant, user), s.Facts.Config.FactsFresh, func(ctx context.Context) ([]byte, error) {
		var r read.UserRelations
		err := s.Facts.read(ctx, tenant, func(ctx context.Context) error {
			var err error
			r, err = s.Reader.Relations(ctx, tenant, user)
			return err
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(r)
	})
	if err != nil {
		return read.UserRelations{}, unavailable(err)
	}
	var r read.UserRelations
	if err := json.Unmarshal(raw, &r); err != nil {
		return read.UserRelations{}, unavailable(err)
	}
	return r, nil
}

// InvalidateRelations implements port.RelationCache: drops the cached relations of the students of a changed block or
// mute (both sides: a block hides both ways, a mute changes Muting of one and MutedBy of the other).
func (s Implicit) InvalidateRelations(ctx context.Context, tenant uuid.UUID, users ...uuid.UUID) error {
	keys := make([]string, len(users))
	for i, u := range users {
		keys[i] = relationsKey(tenant, u)
	}
	return s.Cache.Invalidate(ctx, keys...)
}
