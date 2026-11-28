class_name ChatHubService
extends RefCounted
## The HUD chat of the persistent channels (F112 US1–US3; plan "ChatHubState", GODOT_ARCHITECTURE §6.1; CHAT_SYSTEM §2,
## §4, §7.1, §8.2): glue between ChatService (F119: channels, messages, joins), ClubService (F111: the student's clubs)
## and SocialService (F110: friends) — it keeps the ChatHubState (tabs, DM peers) and the HUD badge up to date:
## - `refresh()`: `listMyChannels` (+ the clubs of the student, the friends) → tabs; called when the hub opens, after a
##   reconnect, on `hub.instance.assigned` (another zone / instance: the old `area` channel is left and its messages
##   dropped — F112-AC-03) and when a blocking activity starts or ends (no "Khu vực" tab outside a hub instance);
## - `open_dm(user_id)` ("Nhắn tin"): `openDirectChannel` through ChatService; `200` / `201` alike; the channel ↔ friend
##   link stays in RAM (DirectPeerMap); `403` / `404` → one neutral sentence and the button hidden for the session
##   (F112-AC-07, AC-08); never called for a friend the student did not pick (F112-AC-10);
## - `can_message(user_id)`: a current friend, `feature_flags.social.friends_enabled` and
##   `policies.chat.direct_message_enabled` not `false`, not refused this session (display only — the server decides);
## - a test / exam (`AppState.blocking_activity` = `exam`) hides the HUD chat and leaves every persistent channel
##   (F112-AC-06); the gameplay chat of F119 is never touched;
## - the badge: unread of the clubs and DMs + the "@" mentions, redrawn at most once a second (F112-AC-05, AC-29).
## No content, nickname or id in logs (AGENTS R14); RAM only, cleared on logout / takeover (F112-AC-31).

## Tabs or their counts changed.
signal tabs_changed()
## HUD badge (`unread`, `mentions`).
signal badge_changed(unread: int, mentions: int)
## The hub must hide / may show again (a test or exam started / ended).
signal availability_changed(available: bool)
## A DM is ready: the hub opens its tab.
signal dm_opened(user_id: String, channel_id: String)
## A channel of a tab left the student (left / removed from the club, history `404`): its tab is gone, no reason shown.
signal channel_removed(channel_id: String)
## A friendly sentence for the hub (`key`, args).
signal notice_requested(key: String, args: Dictionary)

const BLOCKING_EXAM: StringName = &"exam"
## CONFIG_DEFAULTS-free coalescing of the badge (≤ 1 redraw per second, F112-AC-29).
const BADGE_COALESCE_MS: int = 1000
const KEY_DM_UNAVAILABLE: String = "ui.chat.dm.unavailable"

var chat: ChatService = null
var clubs: ClubService = null
var social: SocialService = null
var hub: HubSync = null
var app_state: Node = null
var state: ChatHubState = ChatHubState.new()
## Friends whose DM the server refused this session ("Nhắn tin" hidden — F112-AC-08).
var dm_refused: Dictionary = {}
var _listed: Array[ChatChannelModel] = []
var _badge_due: bool = false
var _badge_sent_ms: int = -BADGE_COALESCE_MS
var _last_badge: Vector2i = Vector2i(-1, -1)
var _available: bool = true


func _init(chat_service: ChatService = null, club_service: ClubService = null, social_service: SocialService = null,
		hub_sync: HubSync = null, app_state_node: Node = null) -> void:
	chat = chat_service
	clubs = club_service
	social = social_service
	hub = hub_sync
	app_state = app_state_node
	if chat != null:
		chat.channels_listed.connect(_on_channels_listed)
		chat.channel_removed.connect(_on_channel_removed)
		chat.store.unread_changed.connect(_on_unread_changed)
		chat.store.messages_changed.connect(_on_messages_changed)
	if hub != null:
		hub.room_assigned.connect(func(_room: HubRoomState) -> void: refresh())
	if app_state != null and app_state.has_signal("blocking_activity_changed"):
		app_state.connect("blocking_activity_changed", _on_blocking_changed)


## The HUD chat may be shown (no test / exam on screen).
func available() -> bool:
	return _available


## Coroutine: `listMyChannels` → tabs (the rebuild runs on ChatService `channels_listed`).
func refresh() -> void:
	if chat == null:
		return
	await chat.load_channels()


## Rebuilds the tabs from the last listed channels (clubs / friends / instance changed).
func rebuild() -> void:
	state.dm_allowed = dm_policy_allows()
	var out: Dictionary = state.rebuild(_listed, chat.store if chat != null else null, _clubs(), _friends(), in_hub())
	for id: String in out["leave"] as PackedStringArray:
		# F112-AC-03: the old area channel is left and its messages leave the RAM.
		if chat != null:
			chat.leave_channel(id)
			chat.store.drop_channel(id)
		channel_removed.emit(id)
	tabs_changed.emit()
	_queue_badge()


## Coroutine: "Nhắn tin" of a friend. True when the DM is ready (its tab opens).
func open_dm(user_id: String) -> bool:
	if chat == null or not can_message(user_id):
		return false
	var out: Dictionary = await chat.open_direct(user_id)
	if not bool(out["ok"]):
		if int(out["status"]) in [403, 404]:
			dm_refused[user_id] = true
			notice_requested.emit(KEY_DM_UNAVAILABLE, {})
			tabs_changed.emit()
		return false
	var channel: ChatChannelModel = out["channel"] as ChatChannelModel
	var card: Dictionary = channel.peer.duplicate() if not channel.peer.is_empty() else _friend_card(user_id)
	card["user_id"] = user_id
	state.peers.remember(channel.channel_id, card)
	state.open_dm(user_id)
	if not _has(channel.channel_id):
		_listed.append(channel)
	rebuild()
	dm_opened.emit(user_id, channel.channel_id)
	return true


## "Nhắn tin" is shown for `user_id` (display only — the server decides, AGENTS R7).
func can_message(user_id: String) -> bool:
	if not Uuid.is_valid(user_id) or dm_refused.has(user_id) or not dm_policy_allows():
		return false
	return social != null and social.store.friend(user_id) != null


## `feature_flags.social.friends_enabled` and `policies.chat.direct_message_enabled` not `false` (absent = shown).
func dm_policy_allows() -> bool:
	if app_state == null:
		return true
	var flags: Dictionary = app_state.get("feature_flags") as Dictionary if app_state.get("feature_flags") is Dictionary else {}
	var social_flags: Variant = flags.get("social")
	if social_flags is Dictionary and (social_flags as Dictionary).get("friends_enabled") is bool \
			and not bool((social_flags as Dictionary)["friends_enabled"]):
		return false
	var policies: Dictionary = app_state.get("school_policies") as Dictionary if app_state.get("school_policies") is Dictionary else {}
	var chat_policy: Variant = policies.get("chat")
	return not (chat_policy is Dictionary and (chat_policy as Dictionary).get("direct_message_enabled") is bool
			and not bool((chat_policy as Dictionary)["direct_message_enabled"]))


## The student is in a hub area (HubSync has a desired area or a room) and no full-screen activity runs (match, boss,
## test…): the server lists the `area` channel of the instance the student stands in.
func in_hub() -> bool:
	if app_state != null and StringName(app_state.get("blocking_activity")) != &"":
		return false
	return hub == null or not hub.desired_area.is_empty() or (hub.room != null and not hub.room.room_id.is_empty())


## Per-frame tick of the HUD: the coalesced badge (≤ 1 per second).
func tick(now_ms: int) -> void:
	if _badge_due and now_ms - _badge_sent_ms >= BADGE_COALESCE_MS:
		_badge_due = false
		_badge_sent_ms = now_ms
		_emit_badge()


## Leaves every persistent channel the hub may have joined (closing the hub, a test / exam starting).
func leave_all() -> void:
	if chat == null:
		return
	for id: String in state.channel_ids():
		chat.leave_channel(id)


func clear() -> void:
	state.clear()
	dm_refused.clear()
	_listed.clear()
	_badge_due = false
	_last_badge = Vector2i(-1, -1)
	badge_changed.emit(0, 0)


func badge() -> Vector2i:
	return Vector2i(state.hud_unread(), state.hud_mentions())


func _on_channels_listed(list: Array[ChatChannelModel]) -> void:
	_listed = list.duplicate()
	rebuild()


func _on_channel_removed(channel_id: String) -> void:
	for i: int in range(_listed.size() - 1, -1, -1):
		if _listed[i].channel_id == channel_id:
			_listed.remove_at(i)
	state.forget_channel(channel_id)
	state.peers.forget(channel_id)
	rebuild()
	channel_removed.emit(channel_id)


func _on_unread_changed() -> void:
	if state.tabs.is_empty():
		return
	state.rebuild(_listed, chat.store, _clubs(), _friends(), in_hub())
	tabs_changed.emit()
	_queue_badge()


func _on_messages_changed(channel_id: String) -> void:
	var ch: ChatChannelModel = chat.store.channel(channel_id)
	if ch == null or ch.channel_type != ChatChannelModel.TYPE_DIRECT or state.peers.knows(channel_id):
		return
	for m: ChatMessageModel in chat.store.visible(channel_id):
		state.peers.from_message(m, ch.channel_type, chat.store.self_id)


func _on_blocking_changed(activity: StringName) -> void:
	var next: bool = activity != BLOCKING_EXAM
	if next != _available:
		_available = next
		if not next:
			leave_all()
		availability_changed.emit(next)
	if not state.tabs.is_empty() or not _listed.is_empty():
		rebuild()


func _queue_badge() -> void:
	_badge_due = true


func _emit_badge() -> void:
	var b: Vector2i = badge()
	if b == _last_badge:
		return
	_last_badge = b
	badge_changed.emit(b.x, b.y)


func _clubs() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	if clubs == null:
		return out
	for c: ClubModel in clubs.store.mine:
		if c.is_member() and not c.chat_channel_id.is_empty():
			out.append({"club_id": c.club_id, "name": c.name, "chat_channel_id": c.chat_channel_id})
	return out


func _friends() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	if social == null:
		return out
	for f: FriendModel in social.store.friends:
		out.append(_card_of(f))
	return out


func _friend_card(user_id: String) -> Dictionary:
	var f: FriendModel = social.store.friend(user_id) if social != null else null
	return _card_of(f) if f != null else {"user_id": user_id}


static func _card_of(f: FriendModel) -> Dictionary:
	return {"user_id": f.user_id, "nickname": f.nickname, "level": f.level, "avatar_config": f.avatar_config}


func _has(channel_id: String) -> bool:
	for c: ChatChannelModel in _listed:
		if c.channel_id == channel_id:
			return true
	return false
