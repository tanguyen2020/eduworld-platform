// Package chat is the chat behaviour of a bot (TEST_STRATEGY §8.1: ~1 message/minute; TC-PERF-010: 6/minute;
// TC-PERF-011 burst): pick a channel the bot can send to (listMyChannels), then send chat.message.send at the
// requested rate and time the ack (chat.message.acked / system.command.acked — includes moderation and the
// JetStream ack, ADR-0035). Bodies are short neutral phrases from a fixed list; nothing from a sensitive-term
// catalogue (AGENTS R15) and nothing is logged.
package chat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/ids"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/gameclient/wsclient"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/report"
	"github.com/tanguyen2020/eduworld-platform/src/backend/tools/loadbot/internal/restcall"
)

// ErrNoChannel means the bot has no channel it may send text to.
var ErrNoChannel = errors.New("chat: no sendable channel")

var phrases = []string{"Xin chào cả nhóm", "Mình sẵn sàng rồi", "Cố lên nào", "Câu này hay quá", "Cảm ơn bạn", "Hẹn gặp lại"}

// PickChannel returns the first channel allowing kind text (listMyChannels).
func PickChannel(ctx context.Context, rc *restcall.Client) (string, error) {
	status, b, err := rc.Do(ctx, "listMyChannels", http.MethodGet, "/chat/channels", nil, "")
	if err != nil {
		return "", fmt.Errorf("chat: listMyChannels: %w", err)
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("chat: listMyChannels: status %d", status)
	}
	var out struct {
		Data []struct {
			ChannelID     string   `json:"channel_id"`
			SendableKinds []string `json:"sendable_kinds"`
		} `json:"data"`
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return "", err
	}
	for _, ch := range out.Data {
		for _, k := range ch.SendableKinds {
			if k == "text" {
				return ch.ChannelID, nil
			}
		}
	}
	return "", ErrNoChannel
}

// Send runs until ctx ends, one message every interval (± 20% jitter); a CHAT_TEMPORARILY_UNAVAILABLE or
// CHAT_RATE_LIMITED failure is counted and the bot keeps its pace (it never retries in a loop).
func Send(ctx context.Context, c *wsclient.Conn, m *report.Metrics, channel string, interval time.Duration, r *rand.Rand) error {
	for {
		wait := time.Duration(float64(interval) * (0.8 + 0.4*r.Float64()))
		select {
		case <-ctx.Done():
			return nil
		case <-c.Done():
			return fmt.Errorf("chat: connection closed")
		case <-time.After(wait):
		}
		_, rtt, err := c.Request(ctx, "chat.message.send", map[string]any{"channel_id": channel, "client_message_id": ids.V4(), "kind": "text",
			"body": phrases[r.IntN(len(phrases))]}, 5*time.Second)
		select {
		case <-ctx.Done():
			return nil // the run ended while the message was in flight: not a chat failure
		default:
		}
		m.WS("chat_send", rtt, err)
		var fe *wsclient.FailedError
		if errors.As(err, &fe) {
			m.Error("chat_" + bounded(fe.Code))
		}
	}
}

// bounded keeps the error label set closed.
func bounded(code string) string {
	switch code {
	case "CHAT_TEMPORARILY_UNAVAILABLE", "CHAT_RATE_LIMITED", "CHAT_CONTENT_BLOCKED", "CHAT_CHANNEL_CLOSED":
		return code
	}
	return "other"
}
