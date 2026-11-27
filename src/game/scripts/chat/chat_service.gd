class_name ChatService
extends GameApiService
## Chat of the game client (F119; GODOT_ARCHITECTURE §6.1, CHAT_SYSTEM §4.2–§9, GAME_NETWORKING §4). The only place of
## the client that talks to the chat API — the panel reads `store` and calls the intents below (AGENTS R1):
## - realtime commands over `/realtime/v1` (`chat.channel.join/leave/sync`, `chat.message.send/edit/recall/forward`,
##   `chat.reaction.toggle`, `chat.pin.add/remove`, `chat.typing.start/stop`, `chat.receipt.delivered/read`); REST
##   `/api/v1/chat/*` for lists, history and the send fallback. Never NATS or Valkey (R25, F119-AC-23);
## - a message is shown as "sent" only after `chat.message.acked` (or the ack / REST answer carrying its server id and
##   seq); every send carries one `client_message_id` kept for every retry — a lost connection or
##   `CHAT_TEMPORARILY_UNAVAILABLE` keeps it pending (clock) and resends with the same id on `client_reconnect_backoff`,
##   so the server never stores it twice (F119-AC-04, AC-05, SC-003);
## - a gameplay channel created a moment after the match / party started answers `NOT_FOUND`: the join is retried on
##   `client_reconnect_backoff`; `CHAT_CHANNEL_CLOSED` locks the composer (F119-AC-06);
## - every moderation decision is the server's: blocked / held / masked / rate limited / muted only become a friendly
##   sentence through ChatFeedbackMapper; the client holds no word list and no moderation rule (R15, CHILD_SAFETY §6);
## - typing (`chat_typing_throttle`) and receipts (`chat_receipt_coalesce`) are throttled before they leave the client.
## No chat content, nickname or token is ever logged (R14): logs carry codes and counts only.

## A friendly notice for the panel (`key` of the catalog, format args, tone of ChatFeedbackMapper).
signal feedback_requested(key: String, args: Dictionary, tone: String)
## "Đang kết nối lại chat…" on (true) / off.
signal connection_changed(reconnecting: bool)
## A refused text message goes back to the composer so the student can rewrite it.
signal draft_returned(channel_id: String, text: String)
## The channel is joined (live messages flow).
signal channel_joined(channel_id: String)
signal quick_messages_loaded()
signal stickers_loaded()
signal gifs_loaded(tag: String)

const PREFIX: String = "chat"
const PATH_CHANNELS: String = "/chat/channels"
const PATH_MESSAGES: String = "/chat/channels/%s/messages"
const PATH_PINS: String = "/chat/channels/%s/pins"
const PATH_QUICK: String = "/chat/quick-messages"
const PATH_STICKERS: String = "/chat/stickers"
const PATH_GIFS: String = "/chat/gifs"
const CMD_JOIN: String = "chat.channel.join"
const CMD_LEAVE: String = "chat.channel.leave"
const CMD_SYNC: String = "chat.channel.sync"
const CMD_SEND: String = "chat.message.send"
const CMD_EDIT: String = "chat.message.edit"
const CMD_RECALL: String = "chat.message.recall"
const CMD_FORWARD: String = "chat.message.forward"
const CMD_REACTION: String = "chat.reaction.toggle"
const CMD_PIN_ADD: String = "chat.pin.add"
const CMD_PIN_REMOVE: String = "chat.pin.remove"
const CMD_TYPING_START: String = "chat.typing.start"
const CMD_TYPING_STOP: String = "chat.typing.stop"
const CMD_DELIVERED: String = "chat.receipt.delivered"
const CMD_READ: String = "chat.receipt.read"
## History page size (`listChannelMessages` limit).
const HISTORY_LIMIT: int = 50
## CONFIG_DEFAULTS `chat_typing_throttle`: one typing signal per 3 s.
const TYPING_THROTTLE_MS: int = 3000
## CONFIG_DEFAULTS `chat_receipt_coalesce`: at most one receipt per 2 s per channel.
const RECEIPT_COALESCE_MS: int = 2000
## STUDENT_GAME_UX_SOCIAL §7 "Khoảng trống seq": nothing is shown when the sync ends within 2 s.
const SYNC_QUIET_MS: int = 2000
## Join attempts of a channel not created yet (`client_reconnect_backoff`, 0.5 s → 15 s, ≈ 1 minute in all).
const JOIN_MAX_ATTEMPTS: int = 8
## Errors after which the same message is sent again later (same `client_message_id`).
const RETRYABLE: PackedStringArray = ["CHAT_TEMPORARILY_UNAVAILABLE", "OFFLINE", "TIMEOUT", "NETWORK_ERROR",
	"UNAVAILABLE", "INTERNAL", "HTTP_502", "HTTP_503", "HTTP_504"]
## A refused text goes back to the composer (rewrite it) — not after a care message.
const RETURN_DRAFT: PackedStringArray = ["CHAT_CONTENT_BLOCKED", "CHAT_RATE_LIMITED", "RATE_LIMITED",
	"VALIDATION_FAILED", "CHAT_MENTION_LIMIT"]
const CODE_NOT_FOUND: String = "NOT_FOUND"
const CODE_CLOSED: String = "CHAT_CHANNEL_CLOSED"
const CODE_MUTED: String = "CHAT_MUTED"
const CODE_BLOCKED: String = "CHAT_CONTENT_BLOCKED"
const KEY_TOO_LONG: String = "ui.chat.feedback.too_long"
const KEY_FORWARDED: String = "ui.chat.forward_done"

var realtime: GameRealtimeClient = null
var app_state: Node = null
var store: ChatStore = ChatStore.new()
var quick: QuickMessageCatalog = QuickMessageCatalog.new()
var assets: ChatAssetCache = null
var budget: ChatAnimationBudget = ChatAnimationBudget.new()
var rng: RandomNumberGenerator = RandomNumberGenerator.new()
## True while messages wait because the chat connection is coming back ("Đang kết nối lại chat…").
var reconnecting: bool = false
## Channels a panel wants (joined again after every reconnect).
var _wanted: Dictionary = {}
var _joined: Dictionary = {}
## channel id → {attempt, next_ms} of a join waiting for the channel to exist.
var _join_retry: Dictionary = {}
var _joining: Dictionary = {}
## client_message_id → send payload (kept until the server answered for good).
var _payloads: Dictionary = {}
## client_message_id → {attempt, next_ms}.
var _retry: Dictionary = {}
var _sending: Dictionary = {}
var _typing_sent: Dictionary = {}
## "channel|delivered" / "channel|read" → highest seq waiting; and the time of the last one sent.
var _receipt_due: Dictionary = {}
var _receipt_sent: Dictionary = {}
## channel id → monotonic ms the sync attempt started.
var _sync_started: Dictionary = {}
var _gifs_loading: Dictionary = {}
var _was_reconnecting: bool = false


func _init(rest: RestApiClient = null, realtime_client: GameRealtimeClient = null, app_state_node: Node = null,
		telemetry_service: TelemetryService = null, cdn_base_url: String = "") -> void:
	super(rest, telemetry_service)
	realtime = realtime_client
	app_state = app_state_node
	var transport: HttpTransport = rest.transport if rest != null else null
	assets = ChatAssetCache.new(transport, cdn_base_url)
	_refresh_self()
	store.gap_detected.connect(_on_gap_detected)
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)
		realtime.connection_state_changed.connect(_on_connection_state_changed)


# ---------- channels ----------

## Coroutine: a panel shows the gameplay channel `channel_id` (party snapshot `chat_channel_id`, match
## `chat_channel_ids`): known at once as an open channel, then the server facts (`listMyChannels`), the newest history
## page and the realtime join (since the newest seq held).
func open_channel(channel_id: String, channel_type: String) -> void:
	_refresh_self()
	var placeholder: ChatChannelModel = ChatChannelModel.placeholder(channel_id, channel_type)
	if placeholder == null:
		return
	if store.channel(channel_id) == null:
		store.ensure_channel(placeholder)
	_wanted[channel_id] = channel_type
	if _joining.has(channel_id):
		return
	_joining[channel_id] = true
	if quick.channel_type != channel_type or not quick.is_loaded():
		load_quick_messages(channel_type)
	await load_channels()
	await load_history(channel_id)
	_joining.erase(channel_id)
	if _wanted.has(channel_id):
		await join(channel_id)


## Coroutine: `listMyChannels` — status, `purge_after`, allowed kinds, features and unread of every channel.
func load_channels() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_CHANNELS), "listMyChannels")
	if result.ok:
		for raw: Variant in DataRead.arr_of(result.body(), "data"):
			var c: ChatChannelModel = ChatChannelModel.from_dict(raw)
			if c != null and (store.channel(c.channel_id) != null or _wanted.has(c.channel_id) or c.channel_type in [
					ChatChannelModel.TYPE_PARTY, ChatChannelModel.TYPE_MATCH, ChatChannelModel.TYPE_TEAM, ChatChannelModel.TYPE_DIRECT]):
				store.ensure_channel(c)
	return result


## Coroutine: one history page (`listChannelMessages`): the newest, or the page before `before_seq` (scrolling up).
func load_history(channel_id: String, before_seq: int = 0) -> ApiResult:
	var query: Dictionary = {"limit": HISTORY_LIMIT}
	if before_seq > 0:
		query["before_seq"] = before_seq
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_MESSAGES % channel_id, query), "listChannelMessages")
	if result.ok:
		var msgs: Array[ChatMessageModel] = _models(DataRead.arr_of(result.body(), "data"))
		var page: Dictionary = DataRead.dict_of(result.body(), "page")
		store.load_history(channel_id, msgs, DataRead.bool_of(page, "has_more"), before_seq > 0)
	return result


## Coroutine: realtime join of the channel. `NOT_FOUND` (the gameplay channel is created right after the event) →
## retried on `client_reconnect_backoff`; `CHAT_CHANNEL_CLOSED` → read-only.
func join(channel_id: String) -> bool:
	if realtime == null or not realtime.can_write():
		return false
	_join_retry.erase(channel_id)
	var outcome: CommandResult = await realtime.send_command(CMD_JOIN, {"channel_id": channel_id,
		"since_seq": store.last_seq(channel_id)})
	if outcome.ok:
		_joined[channel_id] = true
		var info: ChatChannelModel = store.channel(channel_id)
		if info != null and store.state(channel_id) != null and not store.state(channel_id).initialized:
			load_history(channel_id)
		channel_joined.emit(channel_id)
		return true
	match outcome.code:
		CODE_NOT_FOUND:
			_schedule_join(channel_id)
		CODE_CLOSED:
			store.set_status(channel_id, ChatChannelModel.STATUS_CLOSED)
			load_channels()
	return false


## The panel closed: no more live messages of this channel (its messages stay until `clear()`).
func leave_channel(channel_id: String) -> void:
	_wanted.erase(channel_id)
	_join_retry.erase(channel_id)
	store.set_viewing(channel_id, false)
	if _joined.has(channel_id):
		_joined.erase(channel_id)
		if realtime != null and realtime.can_write():
			realtime.send_fire_and_forget(CMD_LEAVE, {"channel_id": channel_id})


func is_joined(channel_id: String) -> bool:
	return _joined.has(channel_id)


func join_pending(channel_id: String) -> bool:
	return _join_retry.has(channel_id)


# ---------- catalogs ----------

## Coroutine: quick messages of `channel_type` (`listQuickMessages`).
func load_quick_messages(channel_type: String) -> ApiResult:
	var query: Dictionary = {"channel_type": channel_type} if not channel_type.is_empty() else {}
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_QUICK, query), "listQuickMessages")
	if result.ok:
		quick.load_list(result.body().get("data"), channel_type)
		quick_messages_loaded.emit()
	return result


## Coroutine: sticker packs of the tenant with the student's unlocks (`listStickerPacks`).
func load_stickers() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_STICKERS), "listStickerPacks")
	if result.ok:
		assets.load_packs(result.body().get("data"))
		stickers_loaded.emit()
	return result


## Coroutine: GIFs of the internal library for a topic tag ("" = first page, which also gives the topics).
func load_gifs(tag: String = "") -> ApiResult:
	if _gifs_loading.has(tag):
		return ApiResult.failure("BUSY")
	_gifs_loading[tag] = true
	var query: Dictionary = {"limit": HISTORY_LIMIT}
	if not tag.is_empty():
		query["tag"] = tag
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_GIFS, query), "listGifs")
	_gifs_loading.erase(tag)
	if result.ok:
		assets.load_gifs(tag, result.body().get("data"))
		gifs_loaded.emit(tag)
	return result


## Coroutine: pinned messages of the channel (`listChannelPins`).
func load_pins(channel_id: String) -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_PINS % channel_id), "listChannelPins")
	if result.ok:
		var ids: Array = []
		for raw: Variant in DataRead.arr_of(result.body(), "data"):
			var m: ChatMessageModel = ChatMessageModel.from_dict(DataRead.dict_of(raw as Dictionary, "message")) if raw is Dictionary else null
			if m != null:
				ids.append(m.message_id)
				store.insert(m, true)
		store.apply_pins(channel_id, ids)
	return result


# ---------- sending (US1, US2, US4) ----------

## A free-text message (Enter / "Gửi"). Returns the pending message, or null when refused before sending (closed,
## too long, locked) — the reason is a notice.
func send_text(channel_id: String, text: String, reply_to: String = "", quote: Dictionary = {},
		mention_ids: PackedStringArray = PackedStringArray()) -> ChatMessageModel:
	var body: String = text.strip_edges()
	var ch: ChatChannelModel = store.channel(channel_id)
	if body.is_empty() or not _can_send(ch, ChatChannelModel.KIND_TEXT):
		return null
	if body.length() > ch.text_limit():
		feedback_requested.emit(KEY_TOO_LONG, {"max": ch.text_limit()}, ChatFeedbackMapper.TONE_INFO)
		return null
	var m: ChatMessageModel = _new_pending(channel_id, ChatChannelModel.KIND_TEXT)
	m.body = body
	var payload: Dictionary = _payload(m, {"body": body})
	if Uuid.is_valid(reply_to):
		m.reply_to_message_id = reply_to
		payload["reply_to_message_id"] = reply_to
		var original: ChatMessageModel = store.find(channel_id, reply_to)
		m.reply_excerpt = ChatText.visible_text(original, quick, assets).left(80) if original != null else ""
	if Uuid.is_valid(String(quote.get("message_id", ""))):
		m.quote_message_id = String(quote["message_id"])
		var q: Dictionary = {"message_id": m.quote_message_id}
		if quote.has("excerpt_start") and quote.has("excerpt_end"):
			q["excerpt_start"] = maxi(0, int(quote["excerpt_start"]))
			q["excerpt_end"] = maxi(int(q["excerpt_start"]), int(quote["excerpt_end"]))
		payload["quote"] = q
	if not mention_ids.is_empty():
		var ids: Array = []
		for id: String in mention_ids:
			if Uuid.is_valid(id) and not id in ids:
				ids.append(id)
		payload["mentions"] = ids
	_submit(m, payload)
	return m


## A quick message in one tap (F119-AC-08): only its id travels; every viewer reads it in their own language.
func send_quick(channel_id: String, quick_message_id: String) -> ChatMessageModel:
	if not _can_send(store.channel(channel_id), ChatChannelModel.KIND_QUICK) or quick_message_id.is_empty():
		return null
	var m: ChatMessageModel = _new_pending(channel_id, ChatChannelModel.KIND_QUICK)
	m.quick_message_id = quick_message_id
	_submit(m, _payload(m, {"quick_message_id": quick_message_id}))
	return m


func send_emoji(channel_id: String, code: String) -> ChatMessageModel:
	if not _can_send(store.channel(channel_id), ChatChannelModel.KIND_EMOJI) or code.is_empty():
		return null
	var m: ChatMessageModel = _new_pending(channel_id, ChatChannelModel.KIND_EMOJI)
	m.emoji = code
	_submit(m, _payload(m, {"emoji": code}))
	return m


func send_sticker(channel_id: String, sticker_id: String) -> ChatMessageModel:
	if not _can_send(store.channel(channel_id), ChatChannelModel.KIND_STICKER) or not Uuid.is_valid(sticker_id):
		return null
	var m: ChatMessageModel = _new_pending(channel_id, ChatChannelModel.KIND_STICKER)
	m.sticker_id = sticker_id
	_submit(m, _payload(m, {"sticker_id": sticker_id}))
	return m


func send_gif(channel_id: String, gif_id: String) -> ChatMessageModel:
	if not _can_send(store.channel(channel_id), ChatChannelModel.KIND_GIF) or not Uuid.is_valid(gif_id):
		return null
	var m: ChatMessageModel = _new_pending(channel_id, ChatChannelModel.KIND_GIF)
	m.gif_id = gif_id
	_submit(m, _payload(m, {"gif_id": gif_id}))
	return m


## Coroutine: edit of the student's own text (`chat.message.edit`, revision check); the new text goes through the
## whole server pipeline again and arrives with `chat.message.edited`. True when the server took it.
func edit(m: ChatMessageModel, text: String) -> bool:
	var body: String = text.strip_edges()
	var ch: ChatChannelModel = store.channel(m.channel_id) if m != null else null
	if m == null or ch == null or not can_edit(m) or body.is_empty():
		return false
	if body.length() > ch.text_limit():
		feedback_requested.emit(KEY_TOO_LONG, {"max": ch.text_limit()}, ChatFeedbackMapper.TONE_INFO)
		return false
	return await _command(CMD_EDIT, {"message_id": m.message_id, "body": body, "revision_expected": m.revision})


## Coroutine: recall of the student's own message (asked to confirm by the panel first, F119-AC-19).
func recall(m: ChatMessageModel) -> bool:
	if m == null or not can_recall(m):
		return false
	var ok: bool = await _command(CMD_RECALL, {"message_id": m.message_id})
	if ok:
		store.apply_gone({"channel_id": m.channel_id, "message_id": m.message_id}, false)
	return ok


## Coroutine: forward to at most 5 channels of the student (each copy goes through the target's pipeline).
func forward(m: ChatMessageModel, target_ids: PackedStringArray) -> bool:
	if m == null or not can_forward(m) or target_ids.is_empty():
		return false
	var ids: Array = []
	for id: String in target_ids:
		if Uuid.is_valid(id) and not id in ids and id != m.channel_id and ids.size() < 5:
			ids.append(id)
	if ids.is_empty():
		return false
	var ok: bool = await _command(CMD_FORWARD, {"message_id": m.message_id, "target_channel_ids": ids})
	if ok:
		feedback_requested.emit(KEY_FORWARDED, {}, ChatFeedbackMapper.TONE_INFO)
	return ok


## Coroutine: reaction on / off (F119-AC-18); counts follow `chat.reaction.updated`, the limit is the server's.
func toggle_reaction(m: ChatMessageModel, code: String) -> bool:
	if m == null or not can_react(m) or code.is_empty():
		return false
	var active: bool = not code in m.my_reactions
	var ok: bool = await _command(CMD_REACTION, {"message_id": m.message_id, "reaction": code})
	if ok:
		store.set_my_reaction(m.message_id, code, active)
	return ok


## Coroutine: pin / unpin (`chat.pin.add/remove`); the bar follows `chat.pin.updated`.
func set_pinned(m: ChatMessageModel, pinned: bool) -> bool:
	if m == null or not can_pin(m):
		return false
	return await _command(CMD_PIN_ADD if pinned else CMD_PIN_REMOVE, {"channel_id": m.channel_id,
		"message_id": m.message_id})


## The student types in the composer: one `chat.typing.start` per 3 s at most.
func typing_started(channel_id: String) -> void:
	var ch: ChatChannelModel = store.channel(channel_id)
	if ch == null or not ch.feature("typing") or realtime == null or not realtime.can_write():
		return
	var now: int = _mono_ms()
	if now - int(_typing_sent.get(channel_id, -TYPING_THROTTLE_MS)) < TYPING_THROTTLE_MS:
		return
	_typing_sent[channel_id] = now
	realtime.send_fire_and_forget(CMD_TYPING_START, {"channel_id": channel_id})


func typing_stopped(channel_id: String) -> void:
	if not _typing_sent.has(channel_id):
		return
	_typing_sent.erase(channel_id)
	if realtime != null and realtime.can_write():
		realtime.send_fire_and_forget(CMD_TYPING_STOP, {"channel_id": channel_id})


## The panel shows `channel_id` (or stops showing it): on screen = read up to the newest seq (`chat.receipt.read`).
func set_viewing(channel_id: String, viewing: bool) -> void:
	store.set_viewing(channel_id, viewing)
	if viewing:
		mark_read(channel_id)


func mark_read(channel_id: String) -> void:
	var ch: ChatChannelModel = store.channel(channel_id)
	if ch == null or ch.channel_type in [ChatChannelModel.TYPE_WORLD, ChatChannelModel.TYPE_AREA]:
		return
	_queue_receipt(channel_id, store.last_seq(channel_id), CMD_READ)


# ---------- permissions shown in the menu (the server checks again) ----------

func can_edit(m: ChatMessageModel) -> bool:
	var ch: ChatChannelModel = store.channel(m.channel_id)
	return ch != null and not ch.is_closed() and ch.feature("edit") and m.is_mine(store.self_id) and m.is_live() \
			and m.kind == ChatChannelModel.KIND_TEXT


func can_recall(m: ChatMessageModel) -> bool:
	var ch: ChatChannelModel = store.channel(m.channel_id)
	return ch != null and not ch.is_closed() and ch.feature("recall") and m.is_mine(store.self_id) \
			and not m.recalled and not m.is_removed() and not m.is_pending() and not m.is_system()


func can_reply(m: ChatMessageModel) -> bool:
	var ch: ChatChannelModel = store.channel(m.channel_id)
	return ch != null and not ch.is_closed() and ch.feature("reply") and ch.allows(ChatChannelModel.KIND_TEXT) and m.is_live()


func can_quote(m: ChatMessageModel) -> bool:
	var ch: ChatChannelModel = store.channel(m.channel_id)
	return ch != null and not ch.is_closed() and ch.feature("quote") and ch.allows(ChatChannelModel.KIND_TEXT) \
			and m.is_live() and m.kind == ChatChannelModel.KIND_TEXT


func can_forward(m: ChatMessageModel) -> bool:
	var ch: ChatChannelModel = store.channel(m.channel_id)
	return ch != null and ch.feature("forward") and m.is_live() and m.moderation_state != ChatMessageModel.MOD_MASKED \
			and not ch.channel_type in [ChatChannelModel.TYPE_WORLD, ChatChannelModel.TYPE_AREA] and not forward_targets(m).is_empty()


func can_react(m: ChatMessageModel) -> bool:
	var ch: ChatChannelModel = store.channel(m.channel_id)
	return ch != null and not ch.is_closed() and ch.feature("reaction") and m.is_live()


func can_pin(m: ChatMessageModel) -> bool:
	var ch: ChatChannelModel = store.channel(m.channel_id)
	return ch != null and not ch.is_closed() and ch.feature("pin") and m.is_live()


## Channels a message can be forwarded to: the student's other open channels except World / Area.
func forward_targets(m: ChatMessageModel) -> Array[ChatChannelModel]:
	var out: Array[ChatChannelModel] = []
	for id: String in store.channel_ids():
		var c: ChatChannelModel = store.channel(id)
		if c != null and id != m.channel_id and not c.is_closed() and not c.channel_type in [ChatChannelModel.TYPE_WORLD,
				ChatChannelModel.TYPE_AREA]:
			out.append(c)
	return out


## The text "Sao chép" puts on the clipboard: what the bubble shows (masked text or placeholder, F119-AC-19).
func copy_text(m: ChatMessageModel) -> String:
	return ChatText.visible_text(m, quick, assets)


## The sync of a gap has run longer than 2 s ("Đang đồng bộ tin nhắn…").
func sync_slow(channel_id: String) -> bool:
	return _sync_started.has(channel_id) and _mono_ms() - int(_sync_started[channel_id]) >= SYNC_QUIET_MS


func pending_count() -> int:
	return _payloads.size()


# ---------- realtime ----------

## Dispatcher handler of `chat.*` (GAME_NETWORKING §4.2).
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	if store.self_id.is_empty():
		_refresh_self()
	match type:
		"chat.message.created":
			var m: ChatMessageModel = ChatMessageModel.from_dict(payload)
			if m != null and store.channel(m.channel_id) != null and store.insert(m) and not m.is_mine(store.self_id):
				_on_received(m)
		"chat.message.acked":
			_acknowledged(payload)
		"chat.message.held":
			var held: ChatMessageModel = store.find(DataRead.str_of(payload, "channel_id"), DataRead.str_of(payload, "message_id"))
			if held != null:
				held.moderation_state = ChatMessageModel.MOD_HELD
				store.message_updated.emit(held.channel_id, held.key())
			var fb: Dictionary = ChatFeedbackMapper.for_held()
			feedback_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))
		"chat.message.edited":
			store.apply_edit(payload)
		"chat.message.recalled":
			store.apply_gone(payload, false)
		"chat.message.removed":
			store.apply_gone(payload, true)
		"chat.reaction.updated":
			store.apply_reactions(DataRead.str_of(payload, "channel_id"), DataRead.str_of(payload, "message_id"),
					payload.get("reactions"))
		"chat.pin.updated":
			store.apply_pins(DataRead.str_of(payload, "channel_id"), DataRead.arr_of(payload, "pinned_message_ids"))
		"chat.typing.updated":
			store.apply_typing(DataRead.str_of(payload, "channel_id"), DataRead.arr_of(payload, "typing"))
		"chat.receipt.updated":
			store.apply_receipt(payload)
		"chat.unread.updated":
			store.apply_unread(DataRead.dict_of(payload, "counts"), DataRead.dict_of(payload, "mentions"))
		"chat.mention.created":
			store.add_mention(DataRead.str_of(payload, "channel_id"))
		"chat.sanction.applied":
			_on_sanction(payload)
		"chat.history.loaded":
			var id: String = DataRead.str_of(payload, "channel_id")
			if store.channel(id) != null:
				store.load_history(id, _models(DataRead.arr_of(payload, "messages")), false)
				_finish_sync(id, DataRead.bool_of(payload, "has_more"))
		"chat.channel.membership_changed":
			var cid: String = DataRead.str_of(payload, "channel_id")
			if store.channel(cid) != null:
				store.set_status(cid, DataRead.str_of(payload, "status", ChatChannelModel.STATUS_OPEN))


## Per-frame tick of the panel host: resends due, join retries, coalesced receipts, sync time-outs.
func tick() -> void:
	var now: int = _mono_ms()
	for cmid: Variant in _retry.keys():
		var r: Dictionary = _retry[cmid] as Dictionary
		if now >= int(r["next_ms"]) and not _sending.has(cmid):
			_send_pending(String(cmid))
	for id: Variant in _join_retry.keys():
		if now >= int((_join_retry[id] as Dictionary)["next_ms"]):
			join(String(id))
	for key: Variant in _receipt_due.keys():
		if now - int(_receipt_sent.get(key, -RECEIPT_COALESCE_MS)) >= RECEIPT_COALESCE_MS:
			_flush_receipt(String(key), now)
	for id: Variant in _sync_started.keys():
		if now - int(_sync_started[id]) >= realtime_timeout_ms():
			_finish_sync(String(id), false)


## Coroutine: the resume failed or the connection came back — join the wanted channels again (they send what was
## missed since the newest seq held) and resend the pending messages at once.
func resync() -> void:
	for id: Variant in _wanted.keys():
		await join(String(id))
	for cmid: Variant in _retry.keys():
		(_retry[cmid] as Dictionary)["next_ms"] = 0
	tick()


## Logout / takeover / new session: nothing of the previous student is kept (data-model §1).
func clear() -> void:
	store.clear()
	quick.clear()
	assets.clear()
	budget.clear()
	_wanted.clear()
	_joined.clear()
	_join_retry.clear()
	_joining.clear()
	_payloads.clear()
	_retry.clear()
	_sending.clear()
	_typing_sent.clear()
	_receipt_due.clear()
	_receipt_sent.clear()
	_sync_started.clear()
	_set_reconnecting(false)


func realtime_timeout_ms() -> int:
	return GameRealtimeClient.COMMAND_TIMEOUT_MS


# ---------- internals ----------

func _can_send(ch: ChatChannelModel, kind: String) -> bool:
	if ch == null:
		return false
	if ch.is_closed():
		var fb: Dictionary = ChatFeedbackMapper.for_code(CODE_CLOSED)
		feedback_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))
		return false
	var lock: String = store.sanction_lock(now_ms())
	if lock == ChatStore.SANCTION_MUTE or (lock == ChatStore.SANCTION_RESTRICT_TEXT and kind != ChatChannelModel.KIND_QUICK):
		var fb: Dictionary = ChatFeedbackMapper.for_code(CODE_MUTED)
		feedback_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))
		return false
	if not ch.allows(kind):
		var fb: Dictionary = ChatFeedbackMapper.for_code("CHAT_KIND_NOT_ALLOWED")
		feedback_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))
		return false
	return true


func _new_pending(channel_id: String, kind: String) -> ChatMessageModel:
	_refresh_self()
	return ChatMessageModel.pending(channel_id, store.self_id, _self_nickname(), kind, now_ms())


func _payload(m: ChatMessageModel, content: Dictionary) -> Dictionary:
	var p: Dictionary = {"channel_id": m.channel_id, "client_message_id": m.client_message_id, "kind": m.kind}
	p.merge(content)
	return p


func _submit(m: ChatMessageModel, payload: Dictionary) -> void:
	store.add_pending(m)
	_payloads[m.client_message_id] = payload
	_send_pending(m.client_message_id)


## Coroutine: one send attempt of a pending message — realtime when the socket takes commands, else the REST fallback
## (`sendChannelMessage`, `Idempotency-Key` = `client_message_id`). The same id on every attempt.
func _send_pending(cmid: String) -> void:
	var payload: Dictionary = _payloads.get(cmid, {}) as Dictionary
	if payload.is_empty() or _sending.has(cmid):
		return
	_sending[cmid] = true
	var code: String = ""
	var category: String = ""
	if realtime != null and realtime.can_write():
		var outcome: CommandResult = await realtime.send_command(CMD_SEND, payload)
		if outcome.ok:
			_sending.erase(cmid)
			var ack: Dictionary = outcome.result.duplicate()
			ack["client_message_id"] = cmid
			ack["channel_id"] = payload["channel_id"]
			if Uuid.is_valid(DataRead.str_of(ack, "message_id")):
				_acknowledged(ack)
			else:
				_retry.erase(cmid)
			return
		code = outcome.code
		category = DataRead.str_of(outcome.details, "category_code")
	else:
		var body: Dictionary = payload.duplicate()
		body.erase("channel_id")
		var req: ApiRequest = ApiRequest.make_post(PATH_MESSAGES % String(payload["channel_id"]), body)
		req.idempotency_key = cmid
		var result: ApiResult = await send_request(req, "sendChannelMessage")
		if result.ok:
			_sending.erase(cmid)
			var b: Dictionary = result.body()
			_acknowledged({"client_message_id": cmid, "message_id": DataRead.str_of(b, "message_id"),
				"channel_id": payload["channel_id"], "seq": b.get("seq"), "moderation_state": DataRead.str_of(b, "moderation_state")})
			return
		code = ApiResult.CODE_NETWORK_ERROR if result.network_error else result.code
		category = DataRead.str_of(result.body(), "category_code")
	_sending.erase(cmid)
	_send_failed(cmid, code, category)


func _send_failed(cmid: String, code: String, category: String) -> void:
	if not _payloads.has(cmid):
		return
	# A gameplay channel created a moment after the party / match started answers NOT_FOUND for a while (CHAT_SYSTEM
	# §4.1): the message waits like a join, at most JOIN_MAX_ATTEMPTS times.
	var early: bool = code == CODE_NOT_FOUND and int((_retry.get(cmid, {"attempt": 0}) as Dictionary)["attempt"]) < JOIN_MAX_ATTEMPTS
	if code in RETRYABLE or early:
		var attempt: int = int((_retry.get(cmid, {"attempt": 0}) as Dictionary)["attempt"]) + 1
		_retry[cmid] = {"attempt": attempt, "next_ms": _mono_ms() + int(Backoff.delay(attempt, rng) * 1000.0)}
		if not early:
			_set_reconnecting(true)
		Log.info("chat_send_retry", {"code": code, "attempt": attempt})
		return
	var channel_id: String = String((_payloads[cmid] as Dictionary).get("channel_id", ""))
	_payloads.erase(cmid)
	_retry.erase(cmid)
	var m: ChatMessageModel = store.remove_pending(cmid)
	if code == CODE_MUTED:
		store.apply_muted()
	elif code == CODE_CLOSED:
		store.set_status(channel_id, ChatChannelModel.STATUS_CLOSED)
		load_channels()
	var fb: Dictionary = ChatFeedbackMapper.for_code(code, category)
	feedback_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))
	Log.info("chat_send_refused", {"code": code})
	if m != null and m.kind == ChatChannelModel.KIND_TEXT and code in RETURN_DRAFT \
			and String(fb["tone"]) != ChatFeedbackMapper.TONE_CARE:
		draft_returned.emit(channel_id, m.body)
	_update_reconnecting()


func _acknowledged(payload: Dictionary) -> void:
	var cmid: String = DataRead.str_of(payload, "client_message_id")
	var m: ChatMessageModel = store.acknowledge(payload)
	if m == null:
		return
	_payloads.erase(cmid)
	_retry.erase(cmid)
	_update_reconnecting()
	if m.moderation_state == ChatMessageModel.MOD_MASKED:
		var fb: Dictionary = ChatFeedbackMapper.for_masked()
		feedback_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))


## Coroutine: a realtime command whose answer is only ok / a friendly notice (offline → "Đang kết nối lại chat…").
func _command(type: String, payload: Dictionary) -> bool:
	if realtime == null or not realtime.can_write():
		var off: Dictionary = ChatFeedbackMapper.for_code(CommandResult.CODE_OFFLINE)
		feedback_requested.emit(String(off["key"]), off["args"] as Dictionary, String(off["tone"]))
		return false
	var outcome: CommandResult = await realtime.send_command(type, payload)
	if outcome.ok:
		return true
	var fb: Dictionary = ChatFeedbackMapper.for_code(outcome.code, DataRead.str_of(outcome.details, "category_code"))
	feedback_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))
	if outcome.code == CODE_MUTED:
		store.apply_muted()
	return false


func _on_received(m: ChatMessageModel) -> void:
	var ch: ChatChannelModel = store.channel(m.channel_id)
	if ch != null and ch.receipts_mode() != ChatChannelModel.RECEIPTS_NONE:
		_queue_receipt(m.channel_id, m.seq, CMD_DELIVERED)
	var st: ChatStore.ChannelState = store.state(m.channel_id)
	if st != null and st.viewing:
		_queue_receipt(m.channel_id, m.seq, CMD_READ)


func _queue_receipt(channel_id: String, seq: int, cmd: String) -> void:
	if seq <= 0:
		return
	var key: String = "%s|%s" % [channel_id, cmd]
	_receipt_due[key] = maxi(int(_receipt_due.get(key, 0)), seq)
	var now: int = _mono_ms()
	if now - int(_receipt_sent.get(key, -RECEIPT_COALESCE_MS)) >= RECEIPT_COALESCE_MS:
		_flush_receipt(key, now)


func _flush_receipt(key: String, now: int) -> void:
	if realtime == null or not realtime.can_write():
		return
	var parts: PackedStringArray = key.split("|")
	if realtime.send_fire_and_forget(parts[1], {"channel_id": parts[0], "seq": int(_receipt_due[key])}):
		_receipt_sent[key] = now
		_receipt_due.erase(key)


func _on_sanction(payload: Dictionary) -> void:
	store.apply_sanction(payload)
	var until: int = WsEnvelope.as_int(payload.get("until"), 0)
	var fb: Dictionary = ChatFeedbackMapper.for_sanction(DataRead.str_of(payload, "sanction_type"),
			DataRead.str_of(payload, "reason_code"), ChatText.time_text(until))
	feedback_requested.emit(String(fb["key"]), fb["args"] as Dictionary, String(fb["tone"]))


## A gap of `count` messages after `after_seq`: one `chat.channel.sync`, or the newest REST page when more than
## `chat_recent_buffer` are missing. Whatever is still missing afterwards is given up (F119-AC-04). The gap is counted
## in telemetry `chat_seq_gap` (channel type + count only, never the channel id — R14, F119 Clarifications 2026-11-28).
func _on_gap_detected(channel_id: String, after_seq: int, count: int) -> void:
	store.begin_sync(channel_id)
	_sync_started[channel_id] = _mono_ms()
	Log.info("chat_gap", {"count": count})
	var gap_channel: ChatChannelModel = store.channel(channel_id)
	if telemetry != null and gap_channel != null:
		telemetry.queue_chat_seq_gap(gap_channel.channel_type, count)
	if count > ChatStore.RECENT_BUFFER or realtime == null or not realtime.can_write():
		await load_history(channel_id)
		_finish_sync(channel_id, false)
		return
	var outcome: CommandResult = await realtime.send_command(CMD_SYNC, {"channels": {channel_id: after_seq}})
	if not outcome.ok:
		await load_history(channel_id)
		_finish_sync(channel_id, false)
	elif DataRead.bool_of(outcome.result, "has_more"):
		await load_history(channel_id)
		_finish_sync(channel_id, false)


func _finish_sync(channel_id: String, has_more: bool) -> void:
	if has_more:
		load_history(channel_id)
	if _sync_started.has(channel_id) or store.is_syncing(channel_id):
		_sync_started.erase(channel_id)
		store.end_sync(channel_id)


func _schedule_join(channel_id: String) -> void:
	var attempt: int = int((_join_retry.get(channel_id, {"attempt": 0}) as Dictionary)["attempt"]) + 1
	if attempt > JOIN_MAX_ATTEMPTS:
		_join_retry.erase(channel_id)
		return
	_join_retry[channel_id] = {"attempt": attempt, "next_ms": _mono_ms() + int(Backoff.delay(attempt, rng) * 1000.0)}


func _models(list: Array) -> Array[ChatMessageModel]:
	var out: Array[ChatMessageModel] = []
	for raw: Variant in list:
		var m: ChatMessageModel = ChatMessageModel.from_dict(raw)
		if m != null:
			out.append(m)
	out.sort_custom(func(a: ChatMessageModel, b: ChatMessageModel) -> bool: return a.seq < b.seq)
	return out


func _set_reconnecting(value: bool) -> void:
	if reconnecting == value:
		return
	reconnecting = value
	connection_changed.emit(value)


func _update_reconnecting() -> void:
	_set_reconnecting(not _retry.is_empty() or (realtime != null and realtime.is_reconnecting()))


func _on_connection_state_changed(_state: int) -> void:
	var now: bool = realtime != null and realtime.is_reconnecting()
	if now:
		_joined.clear()
	_set_reconnecting(now or not _retry.is_empty())
	if _was_reconnecting and not now and realtime != null and realtime.is_online():
		resync()
	_was_reconnecting = now


## Server time in ms (realtime clock when known) — sanction ends, retention days.
func now_ms() -> int:
	if realtime != null and realtime.server_offset_ms != 0:
		return realtime.server_now_ms()
	if api != null and api.clock != null:
		return int(api.clock.unix_time() * 1000.0)
	return int(Time.get_unix_time_from_system() * 1000.0)


func _mono_ms() -> int:
	if api != null and api.clock != null:
		return api.clock.now_msec()
	return Time.get_ticks_msec()


func _refresh_self() -> void:
	var player: Dictionary = app_state.get("player") as Dictionary if app_state != null and app_state.get("player") is Dictionary else {}
	for path: Array in [["student", "student_id"], ["game_profile", "student_id"], ["user", "user_id"]]:
		var id: String = DataRead.str_of(DataRead.dict_of(player, String(path[0])), String(path[1]))
		if Uuid.is_valid(id):
			store.self_id = id
			return


func _self_nickname() -> String:
	var player: Dictionary = app_state.get("player") as Dictionary if app_state != null and app_state.get("player") is Dictionary else {}
	return DataRead.str_of(DataRead.dict_of(player, "game_profile"), "nickname")
