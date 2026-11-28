class_name JoinFriendService
extends RefCounted
## "Tham gia bạn" (F111 US1; SOCIAL_SYSTEM §4.2, MULTIPLAYER_GAMEPLAY §6, F091): moves the student next to a friend who
## is in a public area, with the LATEST `area_id` of `presence.friend.updated` (FriendsStore):
## - same area as the world shows → `HubSync.follow` (one `hub.instance.join` with `follow_user_id`);
## - another area → `SceneRouter.request_area(area_id, {follow_user_id})` (F071 `enterWorldArea`, zone load) and the hub
##   join of the new zone carries `follow_user_id` (WorldRoot → HubSync.enter_area);
## - `hub.instance.assigned` → "Em đã đến chỗ {nickname}" (the world places the student at `spawn`, F092);
## - a refused follow: `ROOM_FULL` → an ordinary join of that area + "Khu vực của bạn đã đầy"; `PERMISSION_DENIED`,
##   `NOT_FOUND` (or any other) → an ordinary join + "Chưa thể tham gia bạn lúc này" — never a room, an area or a reason
##   (AGENTS R16); a refused zone change (`403 AREA_LOCKED`) keeps the student where they are (the world shows the unlock
##   conditions, F071) and no hub join is sent.
## One join at a time: while it is on its way the button is locked (`is_busy`, reason `pending`). The REST
## `joinFriendInWorld` is never called (removed from the contract, contracts §6). UI only reads `availability` and calls
## `join` (AGENTS R1); ids only in logs, never a nickname (R14).

signal join_succeeded(nickname: String)
signal join_failed(code: String)
## A friendly sentence for the HUD / the friends screen (`key` + format args).
signal notice_requested(key: String, args: Dictionary)
signal busy_changed(busy: bool)

const IDLE: String = "idle"
const ENTERING_AREA: String = "entering_area"
const JOINING: String = "joining"
const CODE_ROOM_FULL: String = "ROOM_FULL"
const CODE_AREA_LOCKED: String = "AREA_LOCKED"
const KEY_JOINED: String = "ui.friends.joined"
const KEY_ROOM_FULL: String = "ui.friends.join_room_full"
const KEY_NOT_NOW: String = "ui.friends.join_not_now"

var hub: HubSync = null
var social: SocialService = null
## `(area_id: String, args: Dictionary) -> String` coroutine: "" when the zone changed, else the refusal code; default
## SceneRouter.request_area (F071).
var area_requester: Callable = Callable()
## `() -> String` area the world shows now; default AppState.current_zone.
var current_area: Callable = Callable()
## `() -> Dictionary` the student's own state for JoinAvailability (offline, blocking activity, exam).
var self_state: Callable = Callable()
var state: String = IDLE
var _friend_id: String = ""
var _nickname: String = ""


func _init(hub_sync: HubSync = null, social_service: SocialService = null) -> void:
	hub = hub_sync
	social = social_service
	if hub != null:
		hub.room_assigned.connect(_on_room_assigned)
		hub.follow_failed.connect(_on_follow_failed)


func is_busy() -> bool:
	return state != IDLE


## Reason the button of friend `user_id` is grey (&"" = it can be pressed).
func availability(user_id: String) -> StringName:
	var me: Dictionary = (self_state.call() as Dictionary).duplicate() if self_state.is_valid() else {}
	me["pending"] = is_busy()
	var f: FriendModel = social.store.friend(user_id) if social != null else null
	return JoinAvailability.evaluate(f, me)


## Coroutine: "Tham gia bạn" on friend `user_id`. False when nothing was started (locked, the friend cannot be joined).
func join(user_id: String) -> bool:
	var reason: StringName = availability(user_id)
	if reason != &"":
		return false
	var f: FriendModel = social.store.friend(user_id)
	_friend_id = user_id
	_nickname = f.nickname
	var here: String = String(current_area.call()) if current_area.is_valid() else AppState.current_zone
	if f.area_id == here:
		_set_state(JOINING)
		if hub == null or not hub.follow(user_id):
			_finish_failed(KEY_NOT_NOW, "NOT_SENT")
		return true
	_set_state(ENTERING_AREA)
	var code: String = await _request_area(f.area_id, {"follow_user_id": user_id})
	if not code.is_empty():
		# F111-AC-05: the student stays; the world explains the lock (SceneRouter.area_change_failed).
		Log.info("join_friend_area_refused", {"code": code})
		_set_state(IDLE)
		join_failed.emit(code)
		return true
	if state == ENTERING_AREA:
		_set_state(JOINING)
	return true


func clear() -> void:
	_set_state(IDLE)
	_friend_id = ""
	_nickname = ""


func _request_area(area_id: String, args: Dictionary) -> String:
	if area_requester.is_valid():
		return String(await area_requester.call(area_id, args))
	var refused: Array[String] = []
	var on_failed: Callable = func(_id: String, code: String, _missing: Array) -> void: refused.append(code)
	SceneRouter.area_change_failed.connect(on_failed)
	var err: Error = await SceneRouter.request_area(area_id, "", null, args)
	SceneRouter.area_change_failed.disconnect(on_failed)
	if err == OK:
		return ""
	return refused[0] if not refused.is_empty() else CODE_AREA_LOCKED


func _on_room_assigned(_room: HubRoomState) -> void:
	if state != JOINING:
		return
	var nick: String = _nickname
	_set_state(IDLE)
	notice_requested.emit(KEY_JOINED, {"nickname": nick})
	join_succeeded.emit(nick)


## F111-AC-04: an ordinary join of the same area, then a general sentence.
func _on_follow_failed(code: String) -> void:
	if state != JOINING:
		return
	Log.info("join_friend_refused", {"code": code})
	if hub != null:
		hub.join()
	_finish_failed(KEY_ROOM_FULL if code == CODE_ROOM_FULL else KEY_NOT_NOW, code)


func _finish_failed(key: String, code: String) -> void:
	_set_state(IDLE)
	notice_requested.emit(key, {})
	join_failed.emit(code)


func _set_state(next: String) -> void:
	var was: bool = is_busy()
	state = next
	if was != is_busy():
		busy_changed.emit(is_busy())
