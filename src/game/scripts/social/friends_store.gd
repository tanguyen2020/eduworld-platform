class_name FriendsStore
extends RefCounted
## RAM store of the friends screen (F110 US4, data-model §3; ADR-0064 — never written to disk, cleared on logout or
## session replaced): the friends of `listFriends` in the server order (grouped Online / Offline on screen, never by
## class, grade or school — ADR-0095), the pending requests, the friend code and the block / mute lists.
## `presence.friend.updated` changes one friend at once but the list redraw is coalesced to at most once a second
## (`flush`, STUDENT_GAME_UX_SOCIAL §6 "gộp sự kiện tối đa 1 lần/giây", F110-AC-20).

signal friends_changed()
signal requests_changed()
signal relations_changed()
signal code_changed()

## The list redraw at most once a second.
const COALESCE_MS: int = 1000

var friends: Array[FriendModel] = []
var friends_loaded: bool = false
var incoming: Array[FriendRequestModel] = []
var outgoing: Array[FriendRequestModel] = []
var blocks: Array[RelationModel] = []
var mutes: Array[RelationModel] = []
var code: FriendCodeModel = null
## No network: the presence of every friend reads "—" (F110-AC-20).
var presence_unknown: bool = false
## Redraws of the list (tests).
var emits: int = 0
var _dirty: bool = false
var _last_emit_ms: int = -COALESCE_MS


func set_friends(list: Array[FriendModel]) -> void:
	friends = list
	friends_loaded = true
	_emit_friends()


func friend(user_id: String) -> FriendModel:
	for f: FriendModel in friends:
		if f.user_id == user_id:
			return f
	return null


func online_friends() -> Array[FriendModel]:
	var out: Array[FriendModel] = []
	for f: FriendModel in friends:
		if f.is_online():
			out.append(f)
	return out


func offline_friends() -> Array[FriendModel]:
	var out: Array[FriendModel] = []
	for f: FriendModel in friends:
		if not f.is_online():
			out.append(f)
	return out


## `presence.friend.updated`: the friend changes at once; the redraw waits for `flush`. Unknown players are ignored.
func apply_presence(user_id: String, status: String, activity: String = "", area_id: String = "") -> bool:
	var f: FriendModel = friend(user_id)
	if f == null:
		return false
	f.set_presence(status)
	f.activity = activity.left(64)
	f.area_id = area_id.left(64)
	_dirty = true
	return true


## Emits `friends_changed` when a presence changed and the last redraw is at least COALESCE_MS old.
func flush(now_ms: int) -> bool:
	if not _dirty or now_ms - _last_emit_ms < COALESCE_MS:
		return false
	_last_emit_ms = now_ms
	_emit_friends()
	return true


func set_presence_unknown(value: bool) -> void:
	if value == presence_unknown:
		return
	presence_unknown = value
	_emit_friends()


func remove_friend(user_id: String) -> void:
	var f: FriendModel = friend(user_id)
	if f != null:
		friends.erase(f)
		_emit_friends()


func set_requests(dir: String, list: Array[FriendRequestModel]) -> void:
	if dir == FriendRequestModel.DIRECTION_OUTGOING:
		outgoing = list
	else:
		incoming = list
	requests_changed.emit()


func remove_request(request_id: String) -> void:
	for r: FriendRequestModel in incoming.duplicate():
		if r.request_id == request_id:
			incoming.erase(r)
	for r: FriendRequestModel in outgoing.duplicate():
		if r.request_id == request_id:
			outgoing.erase(r)
	requests_changed.emit()


func add_outgoing(r: FriendRequestModel) -> void:
	if r == null:
		return
	for o: FriendRequestModel in outgoing:
		if o.request_id == r.request_id:
			return
	outgoing.append(r)
	requests_changed.emit()


func set_relations(kind: String, list: Array[RelationModel]) -> void:
	if kind == RelationModel.KIND_MUTE:
		mutes = list
	else:
		blocks = list
	relations_changed.emit()


func remove_relation(kind: String, user_id: String) -> void:
	var list: Array[RelationModel] = mutes if kind == RelationModel.KIND_MUTE else blocks
	for r: RelationModel in list.duplicate():
		if r.user_id == user_id:
			list.erase(r)
	relations_changed.emit()


func set_code(value: FriendCodeModel) -> void:
	code = value
	code_changed.emit()


func clear() -> void:
	friends = []
	friends_loaded = false
	incoming = []
	outgoing = []
	blocks = []
	mutes = []
	code = null
	presence_unknown = false
	_dirty = false
	_emit_friends()
	requests_changed.emit()
	relations_changed.emit()
	code_changed.emit()


func _emit_friends() -> void:
	_dirty = false
	emits += 1
	friends_changed.emit()
