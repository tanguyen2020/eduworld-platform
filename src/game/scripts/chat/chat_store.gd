class_name ChatStore
extends RefCounted
## Client chat state in RAM per channel (F119 US1/US4; GODOT_ARCHITECTURE §6.1, CHAT_SYSTEM §5.4, §8). Pure: no
## network, no Node — ChatService feeds it and the panel reads it. It decides nothing about content (AGENTS R1, R15):
## - messages are ordered by `seq` only (never by arrival); a message without a server `seq` is never listed except the
##   student's own pending ones (clock icon), which become "sent" only on `chat.message.acked` (F119-AC-04, R5);
## - a `seq` above the contiguous cursor + 1 is a gap: `gap_detected` asks the service for one `chat.channel.sync`
##   (or the REST history when more than `chat_recent_buffer` are missing); once that attempt finished the cursor moves
##   past whatever is still missing — a gap the server cannot fill (a held message of someone else, a send that failed
##   after its seq was taken) is tried once only (CHAT_SYSTEM §5.2, GODOT_ARCHITECTURE §6.1);
## - at most `chat_recent_buffer` messages are kept per channel (oldest dropped, F119-AC-21);
## - the student's own messages move pending → sent → delivered → read from the receipts of the other members;
## - blocked / muted players: their messages leave the visible list at once after the server confirmed the block or
##   mute (F119-AC-15) — the server already filters fan-out and history, this only hides what is on screen;
## - a recalled / removed message keeps no content (placeholder only, ADR-0034).
## Nothing is written to disk; `clear()` at logout / takeover.

## Status, unread, pins or title of a channel changed.
signal channel_changed(channel_id: String)
## Messages were added, removed or reordered (the list re-binds its entries).
signal messages_changed(channel_id: String)
## One message changed in place (edit, reactions, receipt state) — only that bubble redraws.
signal message_updated(channel_id: String, key: String)
## `count` messages after `after_seq` are missing: the service makes one sync attempt.
signal gap_detected(channel_id: String, after_seq: int, count: int)
signal typing_changed(channel_id: String)
signal receipts_changed(channel_id: String)
signal unread_changed()
signal hidden_changed()
## `chat.sanction.applied` / `CHAT_MUTED`: the composer locks (or unlocks when the sanction ends).
signal sanction_changed()

## CONFIG_DEFAULTS `chat_recent_buffer` (200, owner VALKEY_DESIGN): messages kept per channel and largest gap filled by
## `chat.channel.sync` (more → REST history).
const RECENT_BUFFER: int = 200
## Nicknames of the typing indicator (CHAT_SYSTEM §7.10: at most 3 shown).
const TYPING_SHOWN: int = 3
const SANCTION_MUTE: String = "mute"
const SANCTION_RESTRICT_TEXT: String = "restrict_text"
## Sanctions that lock the composer of the chat panel (MODERATION §7); the others concern other features.
const LOCKING_SANCTIONS: PackedStringArray = [SANCTION_MUTE, SANCTION_RESTRICT_TEXT]


class ChannelState:
	extends RefCounted
	var channel: ChatChannelModel = null
	## Acknowledged messages, ascending `seq`.
	var messages: Array[ChatMessageModel] = []
	## The student's messages waiting for `chat.message.acked` (creation order).
	var pending: Array[ChatMessageModel] = []
	var by_id: Dictionary = {}
	var by_seq: Dictionary = {}
	## Every seq ≤ contiguous_seq is held, given up after one sync attempt, or older than the history.
	var contiguous_seq: int = 0
	var max_seq: int = 0
	## The first history page arrived (no gap check before: the start of the list is unknown).
	var initialized: bool = false
	var syncing: bool = false
	## Older messages exist on the server (history page `has_more`, or the buffer dropped some).
	var has_older: bool = false
	var receipts: Dictionary = {}
	## `{user_id, nickname}` of the members typing now.
	var typing: Array[Dictionary] = []
	var pins: PackedStringArray = []
	var viewing: bool = false


var self_id: String = ""
## user id → true: players the student blocked or muted in this session (server confirmed).
var hidden: Dictionary = {}
## Active chat sanction of the student: `{type, until_ms, reason_code}`; empty = none.
var sanction: Dictionary = {}
var _channels: Dictionary = {}


# ---------- channels ----------

## Adds the channel or merges the server facts into the one held. Returns its state.
func ensure_channel(model: ChatChannelModel) -> ChannelState:
	if model == null:
		return null
	var st: ChannelState = _channels.get(model.channel_id) as ChannelState
	if st == null:
		st = ChannelState.new()
		st.channel = model
		_channels[model.channel_id] = st
	elif st.channel != model:
		st.channel.merge(model)
	if st.viewing:
		# The channel on screen is read: a list answer counted before the student looked does not bring a badge back.
		st.channel.unread_count = 0
		st.channel.unread_mentions = 0
	channel_changed.emit(model.channel_id)
	return st


func state(channel_id: String) -> ChannelState:
	return _channels.get(channel_id) as ChannelState


func channel(channel_id: String) -> ChatChannelModel:
	var st: ChannelState = state(channel_id)
	return st.channel if st != null else null


func channel_ids() -> PackedStringArray:
	return PackedStringArray(_channels.keys())


func set_status(channel_id: String, status: String, purge_after_ms: int = -1) -> void:
	var st: ChannelState = state(channel_id)
	if st == null:
		return
	st.channel.status = status
	if purge_after_ms >= 0:
		st.channel.purge_after_ms = purge_after_ms
	channel_changed.emit(channel_id)


# ---------- messages ----------

## The list the panel shows: acknowledged messages by `seq`, then the student's pending ones; messages of hidden
## players are left out (the student's own never are).
func visible(channel_id: String) -> Array[ChatMessageModel]:
	var out: Array[ChatMessageModel] = []
	var st: ChannelState = state(channel_id)
	if st == null:
		return out
	for m: ChatMessageModel in st.messages:
		if not _hidden_message(m):
			out.append(m)
	out.append_array(st.pending)
	return out


func find(channel_id: String, message_id: String) -> ChatMessageModel:
	var st: ChannelState = state(channel_id)
	return st.by_id.get(message_id) as ChatMessageModel if st != null else null


## The message `message_id` in any channel.
func find_any(message_id: String) -> ChatMessageModel:
	for st: ChannelState in _channels.values():
		if st.by_id.has(message_id):
			return st.by_id[message_id] as ChatMessageModel
	return null


func add_pending(m: ChatMessageModel) -> void:
	var st: ChannelState = state(m.channel_id)
	if st == null or m.client_message_id.is_empty():
		return
	st.pending.append(m)
	messages_changed.emit(m.channel_id)


## Every pending message of the student (resent after a reconnect with the same `client_message_id`).
func all_pending() -> Array[ChatMessageModel]:
	var out: Array[ChatMessageModel] = []
	for st: ChannelState in _channels.values():
		out.append_array(st.pending)
	return out


func find_pending(client_message_id: String) -> ChatMessageModel:
	for st: ChannelState in _channels.values():
		for m: ChatMessageModel in st.pending:
			if m.client_message_id == client_message_id:
				return m
	return null


## The server refused the message for good (content, rate, mute, closed): it leaves the list.
func remove_pending(client_message_id: String) -> ChatMessageModel:
	for st: ChannelState in _channels.values():
		for i: int in st.pending.size():
			var m: ChatMessageModel = st.pending[i]
			if m.client_message_id == client_message_id:
				st.pending.remove_at(i)
				messages_changed.emit(m.channel_id)
				return m
	return null


## `chat.message.acked` (or the result of the send command / REST answer): the pending message becomes "sent" with
## its server id and `seq`. Unknown `client_message_id` (another device of the student) → ignored. Returns the message.
func acknowledge(payload: Dictionary) -> ChatMessageModel:
	var cmid: String = DataRead.str_of(payload, "client_message_id")
	var mid: String = DataRead.str_of(payload, "message_id")
	var seq: int = WsEnvelope.as_int(payload.get("seq"), 0)
	var m: ChatMessageModel = find_pending(cmid)
	if m == null or not Uuid.is_valid(mid) or seq <= 0:
		return null
	var st: ChannelState = state(m.channel_id)
	st.pending.erase(m)
	m.message_id = mid
	m.seq = seq
	m.local_state = ChatMessageModel.LocalState.SENT
	m.moderation_state = DataRead.str_of(payload, "moderation_state", ChatMessageModel.MOD_ALLOWED)
	var existing: ChatMessageModel = st.by_id.get(mid) as ChatMessageModel
	if existing != null:
		# `chat.message.created` of the same message arrived first: keep that copy (server content).
		existing.client_message_id = cmid
		existing.local_state = ChatMessageModel.LocalState.SENT
		messages_changed.emit(m.channel_id)
		return existing
	_insert(st, m, true)
	messages_changed.emit(m.channel_id)
	return m


## A message from `chat.message.created` (live) or a history page. Returns true when it was new.
func insert(m: ChatMessageModel, from_history: bool = false) -> bool:
	if m == null:
		return false
	var st: ChannelState = state(m.channel_id)
	if st == null:
		return false
	var existing: ChatMessageModel = st.by_id.get(m.message_id) as ChatMessageModel
	if existing != null:
		if m.revision > existing.revision or (m.recalled and not existing.recalled):
			_copy_content(existing, m)
			message_updated.emit(m.channel_id, existing.key())
		return false
	if m.is_mine(self_id):
		m.local_state = ChatMessageModel.LocalState.SENT
	var added: bool = _insert(st, m, from_history)
	if added and not from_history and not m.is_mine(self_id) and not st.viewing and not _hidden_message(m):
		st.channel.unread_count += 1
		unread_changed.emit()
	if added:
		messages_changed.emit(m.channel_id)
	return added


## A history page (REST `listChannelMessages`, `chat.history.loaded`): everything up to its newest seq is known.
func load_history(channel_id: String, messages: Array[ChatMessageModel], has_more: bool = false, older: bool = false) -> void:
	var st: ChannelState = state(channel_id)
	if st == null:
		return
	var newest: int = 0
	for m: ChatMessageModel in messages:
		if m.channel_id != channel_id:
			continue
		newest = maxi(newest, m.seq)
		if not st.by_id.has(m.message_id):
			if m.is_mine(self_id):
				m.local_state = ChatMessageModel.LocalState.SENT
			_insert(st, m, true)
	if older:
		st.has_older = has_more
	elif not st.initialized:
		st.has_older = has_more or st.has_older
	st.initialized = true
	st.contiguous_seq = maxi(st.contiguous_seq, newest)
	_advance(st)
	_refresh_receipt_states(st)
	messages_changed.emit(channel_id)


## The service starts the one sync attempt of a gap.
func begin_sync(channel_id: String) -> void:
	var st: ChannelState = state(channel_id)
	if st != null:
		st.syncing = true


## The sync attempt finished (filled or not): what is still missing is given up (tried once only, F119-AC-04).
func end_sync(channel_id: String) -> void:
	var st: ChannelState = state(channel_id)
	if st == null:
		return
	st.syncing = false
	st.initialized = true
	st.contiguous_seq = maxi(st.contiguous_seq, st.max_seq)


func is_syncing(channel_id: String) -> bool:
	var st: ChannelState = state(channel_id)
	return st != null and st.syncing


## Highest seq held (the `since_seq` / `last_seen_seq` of join and sync).
func last_seq(channel_id: String) -> int:
	var st: ChannelState = state(channel_id)
	return st.max_seq if st != null else 0


func contiguous_seq(channel_id: String) -> int:
	var st: ChannelState = state(channel_id)
	return st.contiguous_seq if st != null else 0


func oldest_seq(channel_id: String) -> int:
	var st: ChannelState = state(channel_id)
	return st.messages[0].seq if st != null and not st.messages.is_empty() else 0


func has_older(channel_id: String) -> bool:
	var st: ChannelState = state(channel_id)
	return st != null and st.has_older and st.messages.size() < RECENT_BUFFER


func count(channel_id: String) -> int:
	var st: ChannelState = state(channel_id)
	return st.messages.size() if st != null else 0


# ---------- message changes (F119 US4) ----------

## `chat.message.edited`: a newer revision replaces the text and mentions (label "đã chỉnh sửa").
func apply_edit(payload: Dictionary) -> void:
	var m: ChatMessageModel = find(DataRead.str_of(payload, "channel_id"), DataRead.str_of(payload, "message_id"))
	var revision: int = WsEnvelope.as_int(payload.get("revision"), 0)
	if m == null or m.recalled or revision <= m.revision:
		return
	m.revision = revision
	m.body = DataRead.str_of(payload, "body").left(ChatMessageModel.BODY_MAX)
	m.edited_at_ms = ChatMessageModel.time_ms(payload.get("edited_at"))
	m.mentions.clear()
	for raw: Variant in DataRead.arr_of(payload, "mentions"):
		if raw is Dictionary and Uuid.is_valid(DataRead.str_of(raw as Dictionary, "user_id")):
			m.mentions[DataRead.str_of(raw as Dictionary, "user_id")] = DataRead.str_of(raw as Dictionary, "nickname")
	message_updated.emit(m.channel_id, m.key())


## `chat.message.recalled` / `chat.message.removed`: placeholder only; replies and quotes of it lose their excerpt
## ("Tin nhắn gốc không còn") and it leaves the pins.
func apply_gone(payload: Dictionary, removed: bool) -> void:
	var channel_id: String = DataRead.str_of(payload, "channel_id")
	var st: ChannelState = state(channel_id)
	var id: String = DataRead.str_of(payload, "message_id")
	if st == null:
		return
	var m: ChatMessageModel = st.by_id.get(id) as ChatMessageModel
	if m != null:
		if removed:
			m.moderation_state = ChatMessageModel.MOD_REMOVED
		else:
			m.recalled = true
		m.clear_content()
	for other: ChatMessageModel in st.messages:
		if other.reply_to_message_id == id:
			other.reply_excerpt = ""
		if other.quote_message_id == id:
			other.quote_excerpt = ""
	if id in st.pins:
		var kept: PackedStringArray = []
		for p: String in st.pins:
			if p != id:
				kept.append(p)
		st.pins = kept
		channel_changed.emit(channel_id)
	messages_changed.emit(channel_id)


## `chat.reaction.updated` (flat counts) or the toggle answer (`{counts, mine}`).
func apply_reactions(channel_id: String, message_id: String, reactions: Variant) -> void:
	var m: ChatMessageModel = find(channel_id, message_id) if not channel_id.is_empty() else find_any(message_id)
	if m == null or m.recalled:
		return
	m.set_reactions(reactions)
	message_updated.emit(m.channel_id, m.key())


## The student's own reaction after the server confirmed the toggle.
func set_my_reaction(message_id: String, code: String, active: bool) -> void:
	var m: ChatMessageModel = find_any(message_id)
	if m == null:
		return
	var had: bool = code in m.my_reactions
	if active and not had:
		m.my_reactions.append(code)
		if int(m.reactions.get(code, 0)) <= 0:
			m.reactions[code] = 1
	elif not active and had:
		var kept: PackedStringArray = []
		for c: String in m.my_reactions:
			if c != code:
				kept.append(c)
		m.my_reactions = kept
	message_updated.emit(m.channel_id, m.key())


## `chat.pin.updated` / `listChannelPins`: the pinned ids (≤ 3) in server order.
func apply_pins(channel_id: String, ids: Array) -> void:
	var st: ChannelState = state(channel_id)
	if st == null:
		return
	st.pins.clear()
	for id: Variant in ids:
		if id is String and Uuid.is_valid(String(id)) and st.pins.size() < 3:
			st.pins.append(String(id))
	for m: ChatMessageModel in st.messages:
		m.pinned = m.message_id in st.pins
	st.channel.pinned_count = st.pins.size()
	channel_changed.emit(channel_id)


func pins(channel_id: String) -> PackedStringArray:
	var st: ChannelState = state(channel_id)
	return st.pins.duplicate() if st != null else PackedStringArray()


## `chat.typing.updated`: the members typing now (never the student, never a hidden player).
func apply_typing(channel_id: String, players: Array) -> void:
	var st: ChannelState = state(channel_id)
	if st == null:
		return
	st.typing.clear()
	for raw: Variant in players:
		if not raw is Dictionary:
			continue
		var id: String = DataRead.str_of(raw as Dictionary, "user_id")
		var nick: String = DataRead.str_of(raw as Dictionary, "nickname").left(ChatMessageModel.NICKNAME_MAX)
		if Uuid.is_valid(id) and id != self_id and not hidden.has(id) and not nick.is_empty():
			st.typing.append({"user_id": id, "nickname": nick})
	typing_changed.emit(channel_id)


## Nicknames to show in the typing indicator (at most 3).
func typing_nicknames(channel_id: String) -> PackedStringArray:
	var out: PackedStringArray = []
	var st: ChannelState = state(channel_id)
	if st == null:
		return out
	for t: Dictionary in st.typing:
		if out.size() >= TYPING_SHOWN:
			break
		out.append(String(t["nickname"]))
	return out


## `chat.receipt.updated`: the cursor of one member moves forward; the student's messages get delivered / read.
func apply_receipt(payload: Dictionary) -> void:
	var st: ChannelState = state(DataRead.str_of(payload, "channel_id"))
	var r: ReceiptStateModel = ReceiptStateModel.from_payload(payload)
	if st == null or r == null or r.member_id == self_id or hidden.has(r.member_id):
		return
	var held: ReceiptStateModel = st.receipts.get(r.member_id) as ReceiptStateModel
	if held == null:
		st.receipts[r.member_id] = r
	else:
		held.advance(r)
	_refresh_receipt_states(st)
	receipts_changed.emit(st.channel.channel_id)


## Who has read / received `m` (tap on the ticks, ≤ 8 members): `{read: [nick], delivered: [nick]}`.
func readers(m: ChatMessageModel) -> Dictionary:
	var read: PackedStringArray = []
	var delivered: PackedStringArray = []
	var st: ChannelState = state(m.channel_id) if m != null else null
	if st != null and m.seq > 0:
		for r: ReceiptStateModel in st.receipts.values():
			if r.read_seq >= m.seq:
				read.append(r.nickname)
			elif r.delivered_seq >= m.seq:
				delivered.append(r.nickname)
	return {"read": read, "delivered": delivered}


# ---------- unread ----------

## `chat.unread.updated` (another screen / device read it): the server counts, except for the channel on screen.
func apply_unread(counts: Dictionary, mentions: Dictionary) -> void:
	for id: Variant in counts.keys():
		var st: ChannelState = state(String(id))
		if st != null and not st.viewing and WsEnvelope.is_int(counts[id]):
			st.channel.unread_count = maxi(0, int(counts[id]))
	for id: Variant in mentions.keys():
		var st: ChannelState = state(String(id))
		if st != null and not st.viewing and WsEnvelope.is_int(mentions[id]):
			st.channel.unread_mentions = maxi(0, int(mentions[id]))
	unread_changed.emit()


## `chat.mention.created` for a channel not on screen: the "@" badge.
func add_mention(channel_id: String) -> void:
	var st: ChannelState = state(channel_id)
	if st != null and not st.viewing:
		st.channel.unread_mentions += 1
		unread_changed.emit()


## The panel shows / stops showing the channel: on screen = read (counts back to 0).
func set_viewing(channel_id: String, viewing: bool) -> void:
	var st: ChannelState = state(channel_id)
	if st == null:
		return
	st.viewing = viewing
	if viewing and (st.channel.unread_count > 0 or st.channel.unread_mentions > 0):
		st.channel.unread_count = 0
		st.channel.unread_mentions = 0
		unread_changed.emit()


func unread(channel_id: String) -> int:
	var c: ChatChannelModel = channel(channel_id)
	return c.unread_count if c != null else 0


func mentions(channel_id: String) -> int:
	var c: ChatChannelModel = channel(channel_id)
	return c.unread_mentions if c != null else 0


# ---------- block / mute, sanction ----------

## The server confirmed a block or a mute: the player's messages leave the list at once (F119-AC-15).
func hide_user(user_id: String) -> void:
	if not Uuid.is_valid(user_id) or user_id == self_id or hidden.has(user_id):
		return
	hidden[user_id] = true
	for st: ChannelState in _channels.values():
		var typing: Array[Dictionary] = []
		for t: Dictionary in st.typing:
			if String(t["user_id"]) != user_id:
				typing.append(t)
		st.typing = typing
		st.receipts.erase(user_id)
		messages_changed.emit(st.channel.channel_id)
	hidden_changed.emit()


func is_hidden(user_id: String) -> bool:
	return hidden.has(user_id)


## `chat.sanction.applied` (MODERATION §7): only the chat sanctions lock the composer; `until` epoch ms (0 = until the
## Safety Team lifts it).
func apply_sanction(payload: Dictionary) -> void:
	var kind: String = DataRead.str_of(payload, "sanction_type")
	sanction = {"type": kind, "until_ms": maxi(0, WsEnvelope.as_int(payload.get("until"), 0)),
		"reason_code": DataRead.str_of(payload, "reason_code").left(64)}
	sanction_changed.emit()


## `CHAT_MUTED` answer without a sanction message: a mute with an unknown end.
func apply_muted() -> void:
	if not sanction.is_empty() and String(sanction.get("type", "")) in LOCKING_SANCTIONS:
		return
	sanction = {"type": SANCTION_MUTE, "until_ms": 0, "reason_code": ""}
	sanction_changed.emit()


## The composer lock now: "" = free, else the sanction type (an ended sanction is dropped).
func sanction_lock(now_ms: int) -> String:
	if sanction.is_empty():
		return ""
	var kind: String = String(sanction.get("type", ""))
	var until: int = int(sanction.get("until_ms", 0))
	if until > 0 and now_ms >= until:
		sanction = {}
		sanction_changed.emit()
		return ""
	return kind if kind in LOCKING_SANCTIONS else ""


# ---------- lifecycle ----------

## Logout / takeover / new session: nothing of the previous student is kept.
func clear() -> void:
	_channels.clear()
	hidden.clear()
	sanction = {}
	self_id = ""
	unread_changed.emit()


## Forgets one channel (the panel left a party / match).
func drop_channel(channel_id: String) -> void:
	_channels.erase(channel_id)
	unread_changed.emit()


# ---------- internals ----------

func _hidden_message(m: ChatMessageModel) -> bool:
	return hidden.has(m.sender_id) and not m.is_mine(self_id)


func _insert(st: ChannelState, m: ChatMessageModel, from_history: bool) -> bool:
	if m.seq <= 0 or st.by_seq.has(m.seq) or st.by_id.has(m.message_id):
		return false
	var at: int = st.messages.size()
	while at > 0 and st.messages[at - 1].seq > m.seq:
		at -= 1
	st.messages.insert(at, m)
	st.by_id[m.message_id] = m
	st.by_seq[m.seq] = m.message_id
	m.pinned = m.pinned or m.message_id in st.pins
	st.max_seq = maxi(st.max_seq, m.seq)
	if st.channel.last_seq < m.seq:
		st.channel.last_seq = m.seq
	if not from_history and st.initialized and not st.syncing and m.seq > st.contiguous_seq + 1:
		var gap_after: int = st.contiguous_seq
		_advance(st)
		gap_detected.emit(st.channel.channel_id, gap_after, m.seq - gap_after - 1)
	else:
		_advance(st)
	while st.messages.size() > RECENT_BUFFER:
		var old: ChatMessageModel = st.messages.pop_front()
		st.by_id.erase(old.message_id)
		st.by_seq.erase(old.seq)
		st.has_older = true
	return true


func _advance(st: ChannelState) -> void:
	while st.by_seq.has(st.contiguous_seq + 1):
		st.contiguous_seq += 1


func _copy_content(into: ChatMessageModel, from: ChatMessageModel) -> void:
	into.revision = from.revision
	into.body = from.body
	into.edited_at_ms = from.edited_at_ms
	into.recalled = from.recalled
	into.moderation_state = from.moderation_state
	into.mentions = from.mentions.duplicate()
	into.reactions = from.reactions.duplicate()
	into.reply_excerpt = from.reply_excerpt
	into.quote_excerpt = from.quote_excerpt
	if into.recalled or into.is_removed():
		into.clear_content()


func _refresh_receipt_states(st: ChannelState) -> void:
	if self_id.is_empty():
		return
	for m: ChatMessageModel in st.messages:
		if not m.is_mine(self_id) or m.seq <= 0:
			continue
		var next: ChatMessageModel.LocalState = ChatMessageModel.LocalState.SENT
		for r: ReceiptStateModel in st.receipts.values():
			if r.read_seq >= m.seq:
				next = ChatMessageModel.LocalState.READ
				break
			if r.delivered_seq >= m.seq:
				next = ChatMessageModel.LocalState.DELIVERED
		if next != m.local_state:
			m.local_state = next
			message_updated.emit(st.channel.channel_id, m.key())
