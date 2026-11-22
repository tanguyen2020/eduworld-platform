package policy

import "time"

// Receipts modes of features.receipts (chat-policy.schema.json v5).
const (
	ReceiptsNone      = "none"
	ReceiptsPerMember = "per_member"
	ReceiptsCount     = "count"
)

// Features is rule.features of the platform level (CHAT_SYSTEM §7.1, F118): which message features a channel type has
// and their limits. Only the platform level switches features (tenant and school documents never loosen nor tighten
// them, CHAT_SYSTEM §7.1); the "always" features of match/party/team are forced on by ApplyAlwaysOn (ADR-0020 §6,
// F118-AC-27). The numbers are CONFIG_DEFAULTS chat_edit_window, chat_recall_window, chat_pins_max, chat_mentions_max,
// chat_forward_targets_max and chat_reactions_per_user, carried by the platform policy document.
type Features struct {
	Edit                bool   `json:"edit"`
	Recall              bool   `json:"recall"`
	Reply               bool   `json:"reply"`
	Quote               bool   `json:"quote"`
	Forward             bool   `json:"forward"`
	Sticker             bool   `json:"sticker"`
	Gif                 bool   `json:"gif"`
	Reaction            bool   `json:"reaction"`
	Mention             bool   `json:"mention"`
	Pin                 bool   `json:"pin"`
	Typing              bool   `json:"typing"`
	Receipts            string `json:"receipts,omitempty"`
	EditWindowMinutes   int    `json:"edit_window_minutes,omitempty"`
	RecallWindowHours   int    `json:"recall_window_hours,omitempty"`
	MaxPins             int    `json:"max_pins,omitempty"`
	MaxMentions         int    `json:"max_mentions,omitempty"`
	MaxForwardTargets   int    `json:"max_forward_targets,omitempty"`
	MaxReactionsPerUser int    `json:"max_reactions_per_user,omitempty"`
}

// Ceilings of the numeric features (database CHECKs and contract maxItems): a document never raises them.
const (
	CeilingPins           = 3
	CeilingMentions       = 5
	CeilingForwardTargets = 5
	CeilingReactions      = 3
	CeilingEditMinutes    = 15
	CeilingRecallHours    = 24
)

func bounded(v, ceiling int) int {
	if v <= 0 || v > ceiling {
		return ceiling
	}
	return v
}

// EditWindow is chat_edit_window (≤ 15 minutes).
func (f Features) EditWindow() time.Duration {
	return time.Duration(bounded(f.EditWindowMinutes, CeilingEditMinutes)) * time.Minute
}

// RecallWindow is chat_recall_window (≤ 24 hours).
func (f Features) RecallWindow() time.Duration {
	return time.Duration(bounded(f.RecallWindowHours, CeilingRecallHours)) * time.Hour
}

// PinsMax is chat_pins_max (≤ 3).
func (f Features) PinsMax() int { return bounded(f.MaxPins, CeilingPins) }

// MentionsMax is chat_mentions_max (≤ 5).
func (f Features) MentionsMax() int { return bounded(f.MaxMentions, CeilingMentions) }

// ForwardTargetsMax is chat_forward_targets_max (≤ 5).
func (f Features) ForwardTargetsMax() int { return bounded(f.MaxForwardTargets, CeilingForwardTargets) }

// ReactionsPerUser is chat_reactions_per_user (≤ 3).
func (f Features) ReactionsPerUser() int { return bounded(f.MaxReactionsPerUser, CeilingReactions) }

// ReceiptMode is the receipts mode (none when unset).
func (f Features) ReceiptMode() string {
	switch f.Receipts {
	case ReceiptsPerMember, ReceiptsCount:
		return f.Receipts
	}
	return ReceiptsNone
}

// alwaysOnFeatures forces the "✓ luôn" features of CHAT_SYSTEM §7.1 on a match/party/team channel: send, sticker/GIF,
// edit, recall, reply/quote, reactions, mention, typing and per-member receipts. Forward and pin stay as configured.
func alwaysOnFeatures(f Features) Features {
	f.Edit, f.Recall, f.Reply, f.Quote, f.Sticker, f.Gif, f.Reaction, f.Mention, f.Typing = true, true, true, true, true, true, true, true, true
	f.Receipts = ReceiptsPerMember
	return f
}

// Feature names of CHAT_FEATURE_DISABLED (no content, metric label).
const (
	FeatureEdit     = "edit"
	FeatureRecall   = "recall"
	FeatureReply    = "reply"
	FeatureQuote    = "quote"
	FeatureForward  = "forward"
	FeatureSticker  = "sticker"
	FeatureGif      = "gif"
	FeatureReaction = "reaction"
	FeatureMention  = "mention"
	FeaturePin      = "pin"
	FeatureTyping   = "typing"
	FeatureReceipts = "receipts"
)

// Has reports a boolean feature by name (receipts: a mode other than none).
func (f Features) Has(name string) bool {
	switch name {
	case FeatureEdit:
		return f.Edit
	case FeatureRecall:
		return f.Recall
	case FeatureReply:
		return f.Reply
	case FeatureQuote:
		return f.Quote
	case FeatureForward:
		return f.Forward
	case FeatureSticker:
		return f.Sticker
	case FeatureGif:
		return f.Gif
	case FeatureReaction:
		return f.Reaction
	case FeatureMention:
		return f.Mention
	case FeaturePin:
		return f.Pin
	case FeatureTyping:
		return f.Typing
	case FeatureReceipts:
		return f.ReceiptMode() != ReceiptsNone
	}
	return false
}
