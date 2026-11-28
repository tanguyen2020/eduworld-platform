class_name ChatChannelModel
extends RefCounted
## One chat channel of the student as the server shows it (F119 data-model §1; OpenAPI `Channel`, CHAT_SYSTEM §2, §7.1).
## Only the display and permission facts the panel needs are kept; the server re-checks every command (AGENTS R1, R5).
## - `channel_type` outside the six known types (a stale server still sending `class`/`study_group`) → null: the client
##   ignores that channel (F119 Clarifications 2026-11-21, ADR-0095).
## - `allowed_kinds` and `features` come from the effective policy of the server. `quick_message_emote_only` (ADR-0078)
##   is simply `allowed_kinds` ⊆ {quick, emoji}: the client has no flag of its own (R15).
## - When a gameplay channel (`match`/`party`/`team`, always-on — ADR-0020 §6) arrives without `features`, the features
##   the platform must keep on for those channels (CHAT_SYSTEM §7.1 "✓ luôn", enforced by the policy schema) are shown;
##   forward and pin stay hidden until the server lists them. Other channel types show nothing without `features`.
## - `max_text_length` is read when the server sends it; otherwise the bound of the contract (`MessageCreate.body`
##   maxLength) caps the composer and the server enforces the policy value (F119 open question, [Proposed]).

const TYPE_WORLD: String = "world"
const TYPE_AREA: String = "area"
const TYPE_MATCH: String = "match"
const TYPE_PARTY: String = "party"
const TYPE_TEAM: String = "team"
const TYPE_DIRECT: String = "direct"
const TYPES: PackedStringArray = [TYPE_WORLD, TYPE_AREA, TYPE_MATCH, TYPE_PARTY, TYPE_TEAM, TYPE_DIRECT]
## Always-on gameplay channels (ADR-0020 §6): no client option can turn them off.
const ALWAYS_ON: PackedStringArray = [TYPE_MATCH, TYPE_PARTY, TYPE_TEAM]
const STATUS_OPEN: String = "open"
const STATUS_CLOSED: String = "closed"
const STATUS_READ_ONLY: String = "read_only"
const STATUS_FROZEN: String = "frozen"
const KIND_TEXT: String = "text"
const KIND_EMOJI: String = "emoji"
const KIND_QUICK: String = "quick"
const KIND_STICKER: String = "sticker"
const KIND_GIF: String = "gif"
const KINDS: PackedStringArray = [KIND_TEXT, KIND_EMOJI, KIND_QUICK, KIND_STICKER, KIND_GIF]
## `MessageCreate.body` / `chat.message.send.body` maxLength of the contract (not a policy value).
const BODY_MAX: int = 300
## CHAT_SYSTEM §7.1 "✓ luôn" for match/party/team (shown when the server sends no `features`).
const ALWAYS_ON_FEATURES: Dictionary = {
	"edit": true, "recall": true, "reply": true, "quote": true, "forward": false, "sticker": true, "gif": true,
	"reaction": true, "mention": true, "pin": false, "typing": true, "receipts": "per_member", "search": false,
}
const RECEIPTS_NONE: String = "none"
const RECEIPTS_PER_MEMBER: String = "per_member"
const RECEIPTS_COUNT: String = "count"

var channel_id: String = ""
var channel_type: String = ""
## Server title (club / direct); gameplay channels are named by the client ("Chat nhóm", "Đội", "Cả trận").
var title: String = ""
var allowed_kinds: PackedStringArray = []
var features: Dictionary = {}
var status: String = STATUS_OPEN
## `purge_after` of a closed gameplay channel (epoch ms, 0 = none) — the retention shown to the student (F119-AC-07).
var purge_after_ms: int = 0
var unread_count: int = 0
var unread_mentions: int = 0
var last_seq: int = 0
var my_read_seq: int = 0
var pinned_count: int = 0
## Server `max_text_length` when sent; 0 = unknown.
var max_text_length: int = 0
## `muted` = the student's own notification setting of the channel (not a way to turn the channel off).
var muted: bool = false
## F112 (F112-AC-12): `sendable_kinds` — what the student may send now (allowed kinds narrowed by a sanction / the DM
## state); `has_sendable` = the server sent the field (absent → `allowed_kinds` decide, delta F119).
var sendable_kinds: PackedStringArray = []
var has_sendable: bool = false
## F112 (F112-5): the public card of the other student of a `direct` channel (`user_id`, `nickname`, `level`,
## `avatar_config` — never a real name, a school or a grade, AGENTS R16); {} when absent.
var peer: Dictionary = {}


## REST `Channel`; null without a valid id or with an unknown `channel_type`.
static func from_dict(data: Variant) -> ChatChannelModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "channel_id")
	var kind: String = DataRead.str_of(d, "channel_type")
	if not Uuid.is_valid(id) or not kind in TYPES:
		return null
	var c: ChatChannelModel = ChatChannelModel.new()
	c.channel_id = id
	c.channel_type = kind
	c.title = DataRead.str_of(d, "title").left(255)
	for k: Variant in DataRead.arr_of(d, "allowed_kinds"):
		if k is String and String(k) in KINDS and not String(k) in c.allowed_kinds:
			c.allowed_kinds.append(String(k))
	if d.get("features") is Dictionary:
		c.features = (d["features"] as Dictionary).duplicate()
	if d.get("sendable_kinds") is Array:
		c.has_sendable = true
		for k: Variant in d["sendable_kinds"] as Array:
			if k is String and String(k) in KINDS and not String(k) in c.sendable_kinds:
				c.sendable_kinds.append(String(k))
	if kind == TYPE_DIRECT and d.get("peer") is Dictionary:
		var p: Dictionary = d["peer"] as Dictionary
		if Uuid.is_valid(DataRead.str_of(p, "user_id")):
			c.peer = {"user_id": DataRead.str_of(p, "user_id"), "nickname": DataRead.str_of(p, "nickname").left(64),
				"level": DataRead.int_of(p, "level"), "avatar_config": DataRead.dict_of(p, "avatar_config")}
	var st: String = DataRead.str_of(d, "status", STATUS_OPEN)
	c.status = st if st in [STATUS_OPEN, STATUS_CLOSED, STATUS_READ_ONLY, STATUS_FROZEN] else STATUS_OPEN
	c.purge_after_ms = ExamCountdown.iso_to_ms(DataRead.str_of(d, "purge_after"))
	c.unread_count = maxi(0, DataRead.int_of(d, "unread_count"))
	c.unread_mentions = maxi(0, DataRead.int_of(d, "unread_mentions"))
	c.last_seq = maxi(0, DataRead.int_of(d, "last_seq"))
	c.my_read_seq = maxi(0, DataRead.int_of(d, "my_read_seq"))
	c.pinned_count = clampi(DataRead.int_of(d, "pinned_count"), 0, 3)
	c.max_text_length = maxi(0, DataRead.int_of(d, "max_text_length"))
	c.muted = DataRead.bool_of(d, "muted")
	return c


## A gameplay channel known only by its id (party snapshot `chat_channel_id`, match `chat_channel_ids`) until
## `listMyChannels` answers: open, the kinds of the always-on rule (CHAT_SYSTEM §2).
static func placeholder(id: String, kind: String) -> ChatChannelModel:
	if not Uuid.is_valid(id) or not kind in TYPES:
		return null
	var c: ChatChannelModel = ChatChannelModel.new()
	c.channel_id = id
	c.channel_type = kind
	if kind in ALWAYS_ON:
		c.allowed_kinds = PackedStringArray([KIND_TEXT, KIND_EMOJI, KIND_QUICK, KIND_STICKER, KIND_GIF])
	return c


## Copies the server facts of `other` (same channel) into this model, keeping the object the screens hold.
func merge(other: ChatChannelModel) -> void:
	if other == null or other.channel_id != channel_id:
		return
	channel_type = other.channel_type
	title = other.title
	allowed_kinds = other.allowed_kinds.duplicate()
	features = other.features.duplicate()
	status = other.status
	purge_after_ms = other.purge_after_ms
	unread_count = other.unread_count
	unread_mentions = other.unread_mentions
	last_seq = maxi(last_seq, other.last_seq)
	my_read_seq = maxi(my_read_seq, other.my_read_seq)
	pinned_count = other.pinned_count
	max_text_length = other.max_text_length
	muted = other.muted
	sendable_kinds = other.sendable_kinds.duplicate()
	has_sendable = other.has_sendable
	if not other.peer.is_empty():
		peer = other.peer.duplicate()


func is_always_on() -> bool:
	return channel_type in ALWAYS_ON


## Writing is closed: the gameplay channel ended, the club is archived or the DM is frozen (reading still works).
func is_closed() -> bool:
	return status != STATUS_OPEN


## F112-AC-12: an allowed kind the student may send now — `allowed_kinds` narrowed by `sendable_kinds` when the server
## sent it (the same rule in every grade band — AGENTS R15).
func allows(kind: String) -> bool:
	return kind in allowed_kinds and (not has_sendable or kind in sendable_kinds)


## Nothing may be sent now (`sendable_kinds` = []): the composer is locked.
func sends_nothing() -> bool:
	return has_sendable and sendable_kinds.is_empty()


## School preset `quick_message_emote_only` (ADR-0078) as the server applied it: no free text, sticker or GIF.
func is_quick_only() -> bool:
	return not allowed_kinds.is_empty() and not allows(KIND_TEXT) and not allows(KIND_STICKER) and not allows(KIND_GIF)


## Effective feature flag (bool features; `receipts` → receipts_mode()).
func feature(name: String) -> bool:
	var source: Dictionary = features if not features.is_empty() else (ALWAYS_ON_FEATURES if is_always_on() else {})
	var v: Variant = source.get(name)
	if v is bool:
		return v as bool
	if v is String:
		return String(v) != RECEIPTS_NONE
	return false


func receipts_mode() -> String:
	var source: Dictionary = features if not features.is_empty() else (ALWAYS_ON_FEATURES if is_always_on() else {})
	var v: Variant = source.get("receipts")
	if v is String and String(v) in [RECEIPTS_NONE, RECEIPTS_PER_MEMBER, RECEIPTS_COUNT]:
		return String(v)
	if v is bool:
		return RECEIPTS_PER_MEMBER if v else RECEIPTS_NONE
	return RECEIPTS_NONE


## Characters a text message may have (the composer counter).
func text_limit() -> int:
	return mini(max_text_length, BODY_MAX) if max_text_length > 0 else BODY_MAX


## Whole days the closed channel stays readable (`purge_after` − now, rounded up, at least 1); 0 = no retention known.
func retention_days(now_ms: int) -> int:
	if purge_after_ms <= 0:
		return 0
	var left: int = purge_after_ms - now_ms
	if left <= 0:
		return 0
	return maxi(1, ceili(float(left) / 86_400_000.0))
