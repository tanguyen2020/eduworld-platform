class_name ChatMessageModel
extends RefCounted
## One chat message (F119 data-model §1): the OpenAPI `Message` (ISO timestamps, reactions `{counts, mine}`, emoji code
## in `body`) or the AsyncAPI `ChatMessage` (epoch-ms timestamps, flat reaction counts, emoji code in `emoji`). Only
## what the server shows is kept: `body` is the moderated text (already masked), never an original; a recalled or
## removed message has no content at all (R25, ADR-0034). The sender is a PlayerCard — nickname and avatar only
## (AGENTS R16); unknown keys are never copied.
## `local_state` is the client view of the student's own messages: pending (clock) → sent (only on
## `chat.message.acked`) → delivered → read (receipts), plus held when the server keeps it for review.

enum LocalState { PENDING, SENT, DELIVERED, READ }

const KIND_SYSTEM: String = "system"
const MOD_ALLOWED: String = "allowed"
const MOD_MASKED: String = "masked"
const MOD_UNCHECKED: String = "unchecked"
const MOD_HELD: String = "held"
const MOD_REMOVED: String = "removed"
const NICKNAME_MAX: int = 32
const BODY_MAX: int = 1000
const EXCERPT_MAX: int = 200

var message_id: String = ""
var client_message_id: String = ""
var channel_id: String = ""
var seq: int = 0
var sender_id: String = ""
var sender_nickname: String = ""
var sender_avatar: Dictionary = {}
var kind: String = ChatChannelModel.KIND_TEXT
## Display text of a text message (masked by the server); "" for other kinds or a recalled/removed message.
var body: String = ""
var emoji: String = ""
var quick_message_id: String = ""
var params: Dictionary = {}
var system_code: String = ""
var sticker_id: String = ""
var gif_id: String = ""
var reply_to_message_id: String = ""
## `reply_excerpt` of the server; empty with a reply id = "Tin nhắn gốc không còn".
var reply_excerpt: String = ""
var quote_message_id: String = ""
## `quote.excerpt` of the server (cut from the moderated text); empty with a quote id = original gone.
var quote_excerpt: String = ""
var forwarded_nickname: String = ""
var is_forwarded: bool = false
## user id → nickname of the mentioned players.
var mentions: Dictionary = {}
## reaction code → count, and the codes of the student (`mine`).
var reactions: Dictionary = {}
var my_reactions: PackedStringArray = []
var revision: int = 1
var edited_at_ms: int = 0
var recalled: bool = false
var pinned: bool = false
var moderation_state: String = MOD_ALLOWED
var created_at_ms: int = 0
var local_state: LocalState = LocalState.SENT


## REST `Message` or AsyncAPI `ChatMessage`; null without the required fields (GAME_NETWORKING §9).
static func from_dict(data: Variant) -> ChatMessageModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var m: ChatMessageModel = ChatMessageModel.new()
	m.message_id = DataRead.str_of(d, "message_id")
	m.channel_id = DataRead.str_of(d, "channel_id")
	m.seq = WsEnvelope.as_int(d.get("seq"), 0)
	var sender: Dictionary = DataRead.dict_of(d, "sender")
	m.sender_id = DataRead.str_of(sender, "user_id")
	m.sender_nickname = DataRead.str_of(sender, "nickname").left(NICKNAME_MAX)
	m.sender_avatar = (sender["avatar_config"] as Dictionary).duplicate(true) if sender.get("avatar_config") is Dictionary else {}
	m.kind = DataRead.str_of(d, "kind")
	if not Uuid.is_valid(m.message_id) or not Uuid.is_valid(m.channel_id) or m.seq <= 0:
		return null
	if not (m.kind in ChatChannelModel.KINDS or m.kind == KIND_SYSTEM):
		return null
	if m.kind != KIND_SYSTEM and (not Uuid.is_valid(m.sender_id) or m.sender_nickname.is_empty()):
		return null
	m.recalled = DataRead.bool_of(d, "recalled")
	m.moderation_state = DataRead.str_of(d, "moderation_state", MOD_ALLOWED)
	var shown: String = DataRead.str_of(d, "body").left(BODY_MAX)
	if m.kind == ChatChannelModel.KIND_EMOJI:
		m.emoji = DataRead.str_of(d, "emoji", shown).left(64)
	elif m.kind == ChatChannelModel.KIND_TEXT:
		m.body = shown
	m.quick_message_id = DataRead.str_of(d, "quick_message_id").left(64)
	m.system_code = DataRead.str_of(d, "system_code").left(128)
	for k: Variant in DataRead.dict_of(d, "params").keys():
		var v: Variant = DataRead.dict_of(d, "params")[k]
		if v is String and m.params.size() < 20:
			m.params[String(k)] = String(v).left(255)
	m.sticker_id = DataRead.str_of(d, "sticker_id")
	m.gif_id = DataRead.str_of(d, "gif_id")
	m.reply_to_message_id = DataRead.str_of(d, "reply_to_message_id")
	m.reply_excerpt = DataRead.str_of(d, "reply_excerpt").left(80)
	var quote: Dictionary = DataRead.dict_of(d, "quote")
	m.quote_message_id = DataRead.str_of(quote, "message_id")
	m.quote_excerpt = DataRead.str_of(quote, "excerpt").left(EXCERPT_MAX)
	if d.get("forwarded_from") is Dictionary:
		m.is_forwarded = true
		m.forwarded_nickname = DataRead.str_of(d["forwarded_from"] as Dictionary, "sender_nickname").left(NICKNAME_MAX)
	for raw: Variant in DataRead.arr_of(d, "mentions"):
		if raw is Dictionary and Uuid.is_valid(DataRead.str_of(raw as Dictionary, "user_id")):
			m.mentions[DataRead.str_of(raw as Dictionary, "user_id")] = DataRead.str_of(raw as Dictionary, "nickname").left(NICKNAME_MAX)
	m.set_reactions(d.get("reactions"))
	m.revision = maxi(1, WsEnvelope.as_int(d.get("revision"), 1))
	m.edited_at_ms = time_ms(d.get("edited_at"))
	m.pinned = DataRead.bool_of(d, "pinned")
	m.created_at_ms = time_ms(d.get("created_at"))
	if m.recalled or m.moderation_state == MOD_REMOVED:
		m.clear_content()
	return m


## A message of the student not acknowledged yet (clock icon): kind + content, a new `client_message_id`.
static func pending(channel: String, self_id: String, nickname: String, msg_kind: String, now_ms: int) -> ChatMessageModel:
	var m: ChatMessageModel = ChatMessageModel.new()
	m.client_message_id = Uuid.v4()
	m.channel_id = channel
	m.sender_id = self_id
	m.sender_nickname = nickname.left(NICKNAME_MAX)
	m.kind = msg_kind
	m.created_at_ms = now_ms
	m.local_state = LocalState.PENDING
	return m


## Epoch ms of an AsyncAPI `EpochMs` or an OpenAPI RFC 3339 timestamp; 0 when absent or invalid.
static func time_ms(value: Variant) -> int:
	if WsEnvelope.is_int(value):
		return maxi(0, int(value))
	if value is String:
		return ExamCountdown.iso_to_ms(value as String)
	return 0


## Reactions as the server sends them: `{counts, mine}` (REST, toggle answer) or a flat `code → count` map (realtime).
## `mine` is kept when a flat map has none.
func set_reactions(value: Variant) -> void:
	if not value is Dictionary:
		return
	var d: Dictionary = value as Dictionary
	var counts: Dictionary = d.get("counts") as Dictionary if d.get("counts") is Dictionary else d
	reactions.clear()
	for k: Variant in counts.keys():
		if k is String and String(k) != "mine" and WsEnvelope.is_int(counts[k]) and int(counts[k]) > 0:
			reactions[String(k)] = int(counts[k])
	if d.get("mine") is Array:
		my_reactions.clear()
		for code: Variant in d["mine"] as Array:
			if code is String and my_reactions.size() < 3:
				my_reactions.append(String(code))
	var kept: PackedStringArray = []
	for code: String in my_reactions:
		if reactions.has(code):
			kept.append(code)
	my_reactions = kept


## Recalled / removed: nothing of the content is kept on the client (ADR-0034, R25).
func clear_content() -> void:
	body = ""
	emoji = ""
	quick_message_id = ""
	params.clear()
	sticker_id = ""
	gif_id = ""
	reply_excerpt = ""
	quote_excerpt = ""
	reactions.clear()
	my_reactions.clear()
	pinned = false


func is_mine(self_id: String) -> bool:
	return not self_id.is_empty() and sender_id == self_id


func is_pending() -> bool:
	return local_state == LocalState.PENDING


func is_removed() -> bool:
	return moderation_state == MOD_REMOVED


func is_held() -> bool:
	return moderation_state == MOD_HELD


func is_system() -> bool:
	return kind == KIND_SYSTEM


## Stable key of the message for the list (server id, else the client id of a pending message).
func key() -> String:
	return message_id if not message_id.is_empty() else client_message_id


## Content that can be quoted, forwarded, reacted to or pinned (not recalled, removed, held or pending).
func is_live() -> bool:
	return not recalled and not is_removed() and not is_held() and not is_pending() and not is_system()
