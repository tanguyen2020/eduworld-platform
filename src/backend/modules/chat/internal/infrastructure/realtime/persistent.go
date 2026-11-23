package realtime

import (
	"context"

	"github.com/google/uuid"

	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/application/port/read"
	"github.com/tanguyen2020/eduworld-platform/src/backend/modules/chat/internal/domain/message"
	rt "github.com/tanguyen2020/eduworld-platform/src/backend/pkg/realtime"
)

// TypeMembershipChanged is chat.channel.membership_changed (AsyncAPI, F120).
const TypeMembershipChanged = "chat.channel.membership_changed"

var (
	_ port.BroadcastFanout = ChatFanout{}
	_ port.MembershipPush  = ChatFanout{}
)

// Broadcast implements port.BroadcastFanout: chat.message.created on ew:chat:fanout:<channel_id> without target — every
// connection the gateway subscribed after the access check of chat.channel.join (World/Area, F120-AC-12, AC-13).
func (f ChatFanout) Broadcast(ctx context.Context, e message.Envelope, sender read.ChatCard) error {
	env, err := rt.NewMessage(TypeMessageCreated, ChatMessage(e, sender, false, port.MessageExtras{}), f.now())
	if err != nil {
		return err
	}
	return f.P.Publish(ctx, rt.ChatFanoutChannel(e.ChannelID.String()), env)
}

// MembershipChanged implements port.MembershipPush on ew:rt:user:<id> (every device of the student; ids and status only).
func (f ChatFanout) MembershipChanged(ctx context.Context, user uuid.UUID, channel uuid.UUID, channelType, op, status string) error {
	env, err := rt.NewMessage(TypeMembershipChanged, map[string]any{"channel_id": channel, "channel_type": channelType, "op": op,
		"status": status}, f.now())
	if err != nil {
		return err
	}
	return f.P.ToUser(ctx, user, env)
}
