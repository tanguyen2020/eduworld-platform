class_name SocialService
extends GameApiService
## Friends, block and mute of the signed-in student (F110 US4, F119-AC-15; SOCIAL_SYSTEM §4, §5, §7; backend F095). The
## only place of the client that calls `/social/*` (AGENTS R1): created by the ServiceLocator with the ApiClient; the
## friends screen and the chat menus read `store` and call the intents below.
## - Friends: `listFriends` (every page, server order; grouped Online / Offline on screen only), `removeFriend`.
## - Requests: `listFriendRequests` (incoming + outgoing), `sendFriendRequest` (from a player card, never a search —
##   there is no API to find a player by name or id, FR-SOC-002), `acceptFriendRequest`, `rejectFriendRequest`. A request
##   this student sent stays "Đang chờ" until it disappears (F110-AC-21).
## - Friend code: `createFriendCode`, `redeemFriendCode` (`404 FRIEND_CODE_INVALID` → "Mã không đúng hoặc đã hết hạn").
## - Block / mute: idempotent `PUT`s, `DELETE` to undo, `listBlocks` / `listMutes`. Once the server answered the client
##   hides the player at once (`relation_changed`: chat messages, the friends list). The player is never told, and no
##   screen says who blocked this student (F110-AC-25).
## - "Mời party" goes through PartyService (F102): `createParty` first when the student has no party (F110-AC-24).
## A `404` on a friend / request / relation (the other player left the school, was removed meanwhile) closes nothing
## loudly: the lists are read again without details (F110-AC-30). Every write carries one Idempotency-Key per intent
## kept while retried (AGENTS R10). Ids only in logs, never a nickname (R14, R16); nothing on disk (ADR-0064).

## The server confirmed: `kind` is "block" or "mute".
signal relation_changed(user_id: String, kind: String)
## A friendly notice for the friends screen (`key` + format args), never a raw code.
signal notice_requested(key: String, args: Dictionary)
## A write started / finished (buttons show the busy state, no double tap).
signal busy_changed(busy: bool)
## A friend / request / relation was gone (`404`): the screen closes the profile and shows the lists read again.
signal refreshed_after_gone()

const KIND_BLOCK: String = "block"
const KIND_MUTE: String = "mute"
const PATH_BLOCK: String = "/social/blocks/%s"
const PATH_MUTE: String = "/social/mutes/%s"
const PATH_BLOCKS: String = "/social/blocks"
const PATH_MUTES: String = "/social/mutes"
const PATH_FRIENDS: String = "/social/friends"
const PATH_FRIEND: String = "/social/friends/%s"
const PATH_REQUESTS: String = "/social/friend-requests"
const PATH_ACCEPT: String = "/social/friend-requests/%s:accept"
const PATH_REJECT: String = "/social/friend-requests/%s:reject"
const PATH_CODES: String = "/social/friend-codes"
const PATH_REDEEM: String = "/social/friend-codes:redeem"
const MSG_NOTIFICATION: String = "notification.item.created"
const KIND_FRIEND_REQUEST: String = "friend_request"
const CODE_CODE_INVALID: String = "FRIEND_CODE_INVALID"
const CODE_RATE_LIMITED: String = "RATE_LIMITED"
const KEY_GENERIC: String = "ui.error.generic"
const KEY_RATE_LIMITED: String = "ui.friends.rate_limited"
## Problem code → friendly sentence (F110-AC-22, AC-23, AC-27: `ui.error.<CODE>`); `RATE_LIMITED` adds the minutes of
## `Retry-After`.
const ERROR_KEYS: Dictionary = {
	"SOCIAL_SCOPE_VIOLATION": "ui.error.SOCIAL_SCOPE_VIOLATION",
	"FRIEND_LIMIT_REACHED": "ui.error.FRIEND_LIMIT_REACHED",
	"FRIEND_CODE_INVALID": "ui.error.FRIEND_CODE_INVALID",
	"PERMISSION_DENIED": "ui.error.PERMISSION_DENIED",
	"CONFLICT": "ui.error.CONFLICT",
	"UNAVAILABLE": "ui.error.UNAVAILABLE",
	"NETWORK_ERROR": "ui.friends.offline",
}

var store: FriendsStore = FriendsStore.new()
## PartyService of the session ("Mời party"); null in tests that do not need it.
var party: PartyService = null
## `() -> int` server/device unix ms for the code countdown (default the device clock).
var now_source: Callable = Callable()
var _intent_keys: Dictionary = {}
var _busy: int = 0


func _init(rest: RestApiClient = null, telemetry_service: TelemetryService = null,
		realtime: GameRealtimeClient = null) -> void:
	super(rest, telemetry_service)
	offline_changed.connect(store.set_presence_unknown)
	if realtime != null:
		realtime.register_handler("notification", handle_notification)


# ---------- friends ----------

## Coroutine: `listFriends` (every page, cursor). Kept as the server sent it — never grouped by class (ADR-0095).
func load_friends() -> ApiResult:
	var pages: Dictionary = await send_all_pages(PATH_FRIENDS, {}, "listFriends")
	if bool(pages["ok"]) or not (pages["items"] as Array).is_empty():
		store.set_friends(FriendModel.list_from(pages["items"]))
	return pages["result"] as ApiResult if pages["result"] != null else ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)


## Coroutine: "Hủy kết bạn" (after the confirmation) — `removeFriend`.
func remove_friend(user_id: String) -> ApiResult:
	if not Uuid.is_valid(user_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var req: ApiRequest = ApiRequest.make_post(PATH_FRIEND % user_id)
	req.method = "DELETE"
	var result: ApiResult = await _write(req, "removeFriend", "")
	if result.ok:
		store.remove_friend(user_id)
	return result


# ---------- requests ----------

## Coroutine: `listFriendRequests` incoming then outgoing (each ≤ 100, server filtered to the pending ones).
func load_requests() -> ApiResult:
	var last: ApiResult = null
	for dir: String in [FriendRequestModel.DIRECTION_INCOMING, FriendRequestModel.DIRECTION_OUTGOING]:
		last = await send_request(ApiRequest.make_get(PATH_REQUESTS, {"direction": dir}), "listFriendRequests")
		if not last.ok:
			return last
		store.set_requests(dir, FriendRequestModel.list_from(last.body().get("data"), dir))
	return last


## Coroutine: "Kết bạn" on a player card / the "Ở gần đây" group — `sendFriendRequest` (`addressee_id`).
func send_friend_request(user_id: String) -> ApiResult:
	if not Uuid.is_valid(user_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var req: ApiRequest = ApiRequest.make_post(PATH_REQUESTS, {"addressee_id": user_id})
	var result: ApiResult = await _write(req, "sendFriendRequest", "send:%s" % user_id)
	if result.ok:
		store.add_outgoing(FriendRequestModel.from_dict(result.body(), FriendRequestModel.DIRECTION_OUTGOING))
		notice_requested.emit("ui.friends.request_sent", {})
	return result


func accept(request: FriendRequestModel) -> ApiResult:
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_ACCEPT % request.request_id), "acceptFriendRequest",
			"accept:%s" % request.request_id)
	if result.ok:
		store.remove_request(request.request_id)
		await load_friends()
	return result


func reject(request: FriendRequestModel) -> ApiResult:
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_REJECT % request.request_id), "rejectFriendRequest", "")
	if result.ok:
		store.remove_request(request.request_id)
	return result


# ---------- friend code ----------

## Coroutine: "Tạo mã" — `createFriendCode` (shown big with its server countdown).
func create_code() -> ApiResult:
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_CODES), "createFriendCode", "code")
	if result.ok:
		store.set_code(FriendCodeModel.from_dict(result.body()))
	return result


## Coroutine: "Gửi" of the code field — `redeemFriendCode` with the normalized code (`^[A-Z0-9]{6,12}$`).
func redeem_code(text: String) -> ApiResult:
	var code: String = FriendCodeModel.normalize(text)
	if not FriendCodeModel.is_valid(code):
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_REDEEM, {"code": code}), "redeemFriendCode",
			"redeem:%s" % code)
	if result.ok:
		store.add_outgoing(FriendRequestModel.from_dict(result.body(), FriendRequestModel.DIRECTION_OUTGOING))
		notice_requested.emit("ui.friends.request_sent", {})
	return result


# ---------- block / mute ----------

## Coroutine: `blockUser` — the player leaves the friends list and the chat at once.
func block_user(user_id: String) -> ApiResult:
	return await _put(user_id, PATH_BLOCK, "blockUser", KIND_BLOCK)


## Coroutine: `muteUser` (hides the player's messages for this student only).
func mute_user(user_id: String) -> ApiResult:
	return await _put(user_id, PATH_MUTE, "muteUser", KIND_MUTE)


func unblock_user(user_id: String) -> ApiResult:
	return await _delete_relation(user_id, PATH_BLOCK, "unblockUser", RelationModel.KIND_BLOCK)


func unmute_user(user_id: String) -> ApiResult:
	return await _delete_relation(user_id, PATH_MUTE, "unmuteUser", RelationModel.KIND_MUTE)


## Coroutine: `listBlocks` + `listMutes` (every page) for the "Đã chặn" tab.
func load_relations() -> ApiResult:
	var last: ApiResult = null
	for pair: Array in [[PATH_BLOCKS, "listBlocks", RelationModel.KIND_BLOCK], [PATH_MUTES, "listMutes", RelationModel.KIND_MUTE]]:
		var pages: Dictionary = await send_all_pages(String(pair[0]), {}, String(pair[1]))
		last = pages["result"] as ApiResult
		if not bool(pages["ok"]):
			return last
		store.set_relations(String(pair[2]), RelationModel.list_from(pages["items"], String(pair[2])))
	return last


# ---------- party ----------

## Coroutine: "Mời party" on a friend's profile — PartyService `inviteToParty`, after `createParty` when the student has
## no party yet (F110-AC-24). The server applies the party rules (grade, school, type) and says why when not (F102).
func invite_to_party(user_id: String) -> ApiResult:
	if party == null:
		return ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)
	if party.store.party == null:
		var created: ApiResult = await party.create_party()
		if not created.ok:
			return created
	var result: ApiResult = await party.invite(user_id)
	if result.ok:
		notice_requested.emit("ui.friends.party_invited", {})
	return result


# ---------- state for the screens ----------

func is_offline() -> bool:
	return offline


func is_busy() -> bool:
	return _busy > 0


func now_ms() -> int:
	if now_source.is_valid():
		return int(now_source.call())
	if api != null and api.clock != null:
		return int(api.clock.unix_time() * 1000.0)
	return int(Time.get_unix_time_from_system() * 1000.0)


## Friendly sentence key of a problem code (F110-AC-22); `RATE_LIMITED` is `ui.friends.rate_limited` with `{minutes}`.
static func error_key(code: String) -> String:
	if code == CODE_RATE_LIMITED:
		return KEY_RATE_LIMITED
	return String(ERROR_KEYS.get(code, KEY_GENERIC))


## Realtime `notification.item.created` of a friend request: the requests are read again and the screen counts follow
## (`EventBus.friend_request_received`). The toast itself is NotificationService's (badge only during a boss phase).
func handle_notification(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	if type != MSG_NOTIFICATION or DataRead.str_of(payload, "kind") != KIND_FRIEND_REQUEST:
		return
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var bus: Node = tree.root.get_node_or_null("EventBus") if tree != null else null
	if bus != null:
		bus.emit_signal("friend_request_received", DataRead.str_of(payload, "ref_id"))
	if store.friends_loaded:
		load_requests()


## Session ended / replaced: nothing of the friends of this student stays (F110-AC-29).
func clear() -> void:
	store.clear()
	_intent_keys.clear()


# ---------- internals ----------

func _put(user_id: String, path: String, operation_id: String, kind: String) -> ApiResult:
	if not Uuid.is_valid(user_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await _write(ApiRequest.make_put(path % user_id), operation_id, "")
	if result.ok:
		if kind == KIND_BLOCK:
			store.remove_friend(user_id)
		relation_changed.emit(user_id, kind)
	return result


func _delete_relation(user_id: String, path: String, operation_id: String, kind: String) -> ApiResult:
	if not Uuid.is_valid(user_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var req: ApiRequest = ApiRequest.make_post(path % user_id)
	req.method = "DELETE"
	var result: ApiResult = await _write(req, operation_id, "")
	if result.ok:
		store.remove_relation(kind, user_id)
	return result


## One REST write: `intent` keeps the Idempotency-Key until the server answered (retry after a network failure = same
## key); a problem becomes a friendly notice, a `404` of a gone player reads the lists again without details.
func _write(req: ApiRequest, operation_id: String, intent: String) -> ApiResult:
	if not intent.is_empty():
		if not _intent_keys.has(intent):
			_intent_keys[intent] = Uuid.v7()
		req.idempotency_key = String(_intent_keys[intent])
	_set_busy(1)
	var result: ApiResult = await send_request(req, operation_id)
	_set_busy(-1)
	if not intent.is_empty() and not result.network_error and not result.is_server_unavailable():
		_intent_keys.erase(intent)
	if result.ok or result.code in ACCESS_DENIED_CODES:
		return result
	if result.status == 404 and result.code != CODE_CODE_INVALID:
		await _refresh_after_gone()
		return result
	var args: Dictionary = {}
	if result.code == CODE_RATE_LIMITED:
		args["minutes"] = maxi(1, ceili(result.retry_after_sec(60.0) / 60.0))
	notice_requested.emit(error_key(result.code), args)
	return result


func _refresh_after_gone() -> void:
	await load_friends()
	await load_requests()
	if not store.blocks.is_empty() or not store.mutes.is_empty():
		await load_relations()
	refreshed_after_gone.emit()


func _set_busy(delta: int) -> void:
	var before: bool = _busy > 0
	_busy = maxi(0, _busy + delta)
	if before != (_busy > 0):
		busy_changed.emit(_busy > 0)
