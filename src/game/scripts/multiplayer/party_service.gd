class_name PartyService
extends GameApiService
## Party of the signed-in student (F102; PARTY_SYSTEM §2, §5, §9; GODOT_ARCHITECTURE §4, §6; GAME_NETWORKING §8). The
## only place of the client that calls the party API (AGENTS R1, F102-AC-20): created by the ServiceLocator with the
## ApiClient and the RealtimeClient (fakes in tests); the lobby, the HUD and the invitation toast read `store` and call
## the intents below.
## - REST writes (`createParty`, `inviteToParty`, `acceptPartyInvitation`, `joinParty`, `transferPartyLeader`,
##   `startPartySession`) carry one `Idempotency-Key` per intent of the student, kept when the same intent is retried
##   after a network failure, dropped once the server answered (AGENTS R10, F102-AC-01).
## - Ready (`party.member.ready_set`) and the activity (`party.activity.select` with `expected_version`) go over the
##   realtime connection when it can take commands, else over REST (`setPartyReady`, `selectPartyActivity`); the screen
##   changes only with what the server confirmed — the push, the ack or the REST answer (F102-AC-08, ADR-0016).
## - Realtime `party.*` messages (dispatcher prefix `party`) go to `PartyStore`; a version gap or `VERSION_MISMATCH`
##   reloads the party with `getParty` (F102-AC-07); the start is never re-sent by itself (spec edge case).
## - While the connection is coming back (or a REST call found no network) the student's writes are locked and the
##   party stays readable; once online again `getMyParty` + the pending invitations are read and, when the party is in a
##   session this student belongs to, the session room is entered again (F102-AC-15).
## - Problems become friendly sentences `ui.party.error.<CODE>` (never the raw code, F102-AC-05).
## The client never checks party rules (grade, school, party type, eligibility): the server does and says why (R1, R5,
## ADR-0095). Nicknames and ids of other students are never logged (R14); nothing is written to disk (ADR-0064).

## A friendly notice for the screens (`key` of the catalog + format args): errors, "Nhóm đã giải tán", new leader…
signal notice_requested(key: String, args: Dictionary)
## The student's writes are locked (no network) or unlocked again.
signal connection_changed(offline: bool)
signal friends_changed()
signal eligible_changed()
## A pending write started / finished (buttons show the busy state, no double tap).
signal busy_changed(busy: bool)

const PREFIX: String = "party"
const PATH_PARTIES: String = "/multiplayer/parties"
const PATH_MY_PARTY: String = "/multiplayer/parties/me"
const PATH_PARTY: String = "/multiplayer/parties/%s"
const PATH_LEAVE: String = "/multiplayer/parties/%s:leave"
const PATH_DISBAND: String = "/multiplayer/parties/%s:disband"
const PATH_TRANSFER: String = "/multiplayer/parties/%s:transfer-leader"
const PATH_KICK: String = "/multiplayer/parties/%s/members/%s:kick"
const PATH_READY: String = "/multiplayer/parties/%s/members/me/ready"
const PATH_ACTIVITY: String = "/multiplayer/parties/%s/activity"
const PATH_START: String = "/multiplayer/parties/%s:start"
const PATH_ELIGIBLE: String = "/multiplayer/parties/%s/eligible-activities"
const PATH_INVITE: String = "/multiplayer/parties/%s/invitations"
const PATH_INVITATIONS: String = "/multiplayer/invitations"
const PATH_ACCEPT: String = "/multiplayer/invitations/%s:accept"
const PATH_REJECT: String = "/multiplayer/invitations/%s:reject"
const PATH_FRIENDS: String = "/social/friends"
const CMD_READY: String = "party.member.ready_set"
const CMD_SELECT: String = "party.activity.select"
const PARTY_TYPE_FRIEND: String = "friend"
## F110: `party.session.started.room_type` of a party boss (the room id is the boss encounter).
const ROOM_BOSS: String = "boss"
## `PartyCreate.name` maxLength (the server also filters the name, PARTY_SYSTEM §3).
const NAME_MAX_LENGTH: int = 40
const CODE_ALREADY_IN_PARTY: String = "PARTY_ALREADY_IN_PARTY"
const CODE_NOT_FOUND: String = "PARTY_NOT_FOUND"
const CODE_EXPIRED: String = "PARTY_INVITATION_EXPIRED"
const CODE_VERSION_MISMATCH: String = "VERSION_MISMATCH"
const CODE_WS_CONFLICT: String = "CONFLICT"
const CODE_OFFLINE: String = "OFFLINE"
const KEY_GENERIC_ERROR: String = "ui.party.error.generic"
## Problem code → friendly sentence (F102-AC-05). Codes outside the list show the generic sentence, never the code.
const ERROR_KEYS: Dictionary = {
	"PARTY_FULL": "ui.party.error.PARTY_FULL",
	"PARTY_INVITATION_EXPIRED": "ui.party.error.PARTY_INVITATION_EXPIRED",
	"PARTY_INVITE_RATE_LIMITED": "ui.party.error.PARTY_INVITE_RATE_LIMITED",
	"PARTY_INVITE_NOT_ALLOWED": "ui.party.error.PARTY_INVITE_NOT_ALLOWED",
	"PARTY_POLICY_VIOLATION": "ui.party.error.PARTY_POLICY_VIOLATION",
	"PARTY_NOT_LEADER": "ui.party.error.PARTY_NOT_LEADER",
	"PARTY_ACTIVITY_NOT_ELIGIBLE": "ui.party.error.PARTY_ACTIVITY_NOT_ELIGIBLE",
	"PARTY_NOT_READY": "ui.party.error.PARTY_NOT_READY",
	"PARTY_ALREADY_IN_PARTY": "ui.party.error.PARTY_ALREADY_IN_PARTY",
	"PARTY_NOT_FOUND": "ui.party.error.PARTY_NOT_FOUND",
	"PARTY_MEMBER_NOT_ONLINE": "ui.party.error.PARTY_MEMBER_NOT_ONLINE",
	"VERSION_MISMATCH": "ui.party.error.VERSION_MISMATCH",
	"CONFLICT": "ui.party.error.VERSION_MISMATCH",
	"RATE_LIMITED": "ui.party.error.RATE_LIMITED",
	"UNAVAILABLE": "ui.party.error.UNAVAILABLE",
	"NETWORK_ERROR": "ui.party.error.NETWORK_ERROR",
	"OFFLINE": "ui.party.error.NETWORK_ERROR",
	"TIMEOUT": "ui.party.error.NETWORK_ERROR",
	"HTTP_410": "ui.party.error.PARTY_INVITATION_EXPIRED",
}
const KEY_DISBANDED: String = "ui.party.disbanded"
const KEY_KICKED: String = "ui.party.kicked"
const KEY_NEW_LEADER: String = "ui.party.new_leader"
const KEY_ACTIVITY_CHANGED: String = "ui.party.activity_changed"
const KEY_DEFAULT_NAME: String = "ui.party.default_name"
## How often the expired invitations are dropped (ms).
const EXPIRE_CHECK_MS: int = 1000

var realtime: GameRealtimeClient = null
var app_state: Node = null
var store: PartyStore = PartyStore.new()
## Settings autoload (or a test stub) with the local popup switch `notification_popups` (F123).
var settings: Object = null
## `() -> bool`: true during a timed exam, a solo challenge or the activity player — no invitation popup then.
var quiet_check: Callable = Callable()
## Friends of `listFriends` (PlayerCard) for the invite picker, in the server order (never filtered by grade/school).
var friends: Array[PartyMemberModel] = []
var friends_loaded: bool = false
## Students invited from this client during the current party (the picker shows "Đã mời").
var invited: Dictionary = {}
var eligible: Array[EligibleActivityModel] = []
## `(scene_ref: String, room_id: String) -> Error`: enters the session room (default SceneRouter.enter_party_session).
var session_router: Callable = Callable()
## F110: `(encounter_id: String, visual_ref: String) -> Error` for a boss session (`room_type = boss`, `room_id` = the
## encounter): default SceneRouter.enter_boss_encounter (the BossArena screen, F110-AC-02).
var boss_router: Callable = Callable()
var _intent_keys: Dictionary = {}
var _busy: int = 0
var _was_reconnecting: bool = false
var _last_expire_ms: int = 0


func _init(rest: RestApiClient = null, realtime_client: GameRealtimeClient = null, app_state_node: Node = null,
		settings_node: Object = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	realtime = realtime_client
	app_state = app_state_node
	settings = settings_node
	_refresh_self()
	store.resync_needed.connect(_on_resync_needed)
	store.party_updated.connect(_on_party_updated)
	store.party_closed.connect(_on_party_closed)
	store.leader_changed.connect(_on_leader_changed)
	store.activity_changed.connect(_on_activity_changed)
	store.invitation_received.connect(_on_invitation_received)
	offline_changed.connect(func(value: bool) -> void: connection_changed.emit(value or _reconnecting()))
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)
		realtime.connection_state_changed.connect(_on_connection_state_changed)


# ---------- reads ----------

## Coroutine: `getMyParty` — entering the world, back online, a failed resume. 404 = no party (closed silently).
func load_my_party() -> ApiResult:
	_refresh_self()
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_MY_PARTY), "getMyParty")
	if result.ok:
		store.apply_snapshot(PartyModel.from_rest(result.body()))
	elif result.status == 404:
		store.close(PartyStore.CLOSED_REMOVED)
	return result


## Coroutine: `getParty` of the party held (version gap, VERSION_MISMATCH).
func refresh_party() -> ApiResult:
	if store.party == null:
		return await load_my_party()
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_PARTY % store.party.party_id), "getParty")
	if result.ok:
		store.apply_snapshot(PartyModel.from_rest(result.body()))
	elif result.status == 404:
		store.close(PartyStore.CLOSED_REMOVED)
	return result


## Coroutine: pending invitations of the student (`listMyInvitations?status=pending`).
func load_invitations() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_INVITATIONS,
			{"status": InvitationModel.STATUS_PENDING}), "listMyInvitations")
	if result.ok:
		store.set_invitations(InvitationModel.list_from(result.body().get("data")))
		store.expire_invitations(now_ms())
	return result


## Coroutine: the friends of the student for the invite picker (`listFriends`, every page). The list is shown as the
## server sent it: no grouping or filter by class, grade or school (ADR-0095, F102-AC-02).
func load_friends() -> ApiResult:
	var pages: Dictionary = await send_all_pages(PATH_FRIENDS, {}, "listFriends")
	var result: ApiResult = pages["result"] as ApiResult
	if bool(pages["ok"]) or not (pages["items"] as Array).is_empty():
		friends.clear()
		for raw: Variant in pages["items"] as Array:
			var f: PartyMemberModel = PartyMemberModel.from_card(raw)
			if f != null:
				friends.append(f)
		friends_loaded = true
		friends_changed.emit()
	return result if result != null else ApiResult.failure(ApiResult.CODE_NETWORK_ERROR)


## Coroutine: activities of the party with the eligibility the server computed (`listPartyEligibleActivities`).
func load_eligible() -> ApiResult:
	if store.party == null:
		return ApiResult.failure(CODE_NOT_FOUND)
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_ELIGIBLE % store.party.party_id),
			"listPartyEligibleActivities")
	if result.ok:
		eligible = EligibleActivityModel.list_from(result.body().get("data"))
		for a: EligibleActivityModel in eligible:
			store.titles[a.activity_ref] = a.title
		if store.party != null and store.party.activity_title.is_empty() and store.titles.has(store.party.activity_id):
			store.party.activity_title = store.titles[store.party.activity_id] as Dictionary
		eligible_changed.emit()
	return result


# ---------- intents (US1) ----------

## Coroutine: `createParty` (F102-AC-01) — a `friend` party named after the student; the same key while it is retried.
func create_party() -> ApiResult:
	if store.party != null:
		return ApiResult.failure(CODE_ALREADY_IN_PARTY)
	var body: Dictionary = {"name": default_name(), "party_type": PARTY_TYPE_FRIEND}
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_PARTIES, body), "createParty", "create")
	if result.ok:
		invited.clear()
		store.apply_snapshot(PartyModel.from_rest(result.body()))
	return result


## Coroutine: `inviteToParty` of a friend chosen in the picker (one key per invited student).
func invite(student_id: String) -> ApiResult:
	if store.party == null:
		return ApiResult.failure(CODE_NOT_FOUND)
	var req: ApiRequest = ApiRequest.make_post(PATH_INVITE % store.party.party_id, {"invitee_id": student_id})
	var result: ApiResult = await _write(req, "inviteToParty", "invite:%s:%s" % [store.party.party_id, student_id])
	if result.ok:
		invited[student_id] = true
		friends_changed.emit()
	return result


## Coroutine: `acceptPartyInvitation` (F102-AC-04). `409 PARTY_ALREADY_IN_PARTY` is returned without a notice: the
## screen asks "Rời nhóm hiện tại và tham gia?" and calls `switch_and_accept` only when the student agrees.
func accept(invitation: InvitationModel) -> ApiResult:
	var req: ApiRequest = ApiRequest.make_post(PATH_ACCEPT % invitation.invitation_id)
	var result: ApiResult = await _write(req, "acceptPartyInvitation", "accept:%s" % invitation.invitation_id,
			PackedStringArray([CODE_ALREADY_IN_PARTY]))
	if result.ok:
		store.remove_invitation(invitation.invitation_id)
		invited.clear()
		store.apply_snapshot(PartyModel.from_rest(result.body()))
	elif result.code == CODE_EXPIRED or result.status == 410 or result.status == 404:
		store.remove_invitation(invitation.invitation_id)
	return result


## Coroutine: the student agreed to leave the current party for `invitation`: `leaveParty`, then a new accept intent.
func switch_and_accept(invitation: InvitationModel) -> ApiResult:
	if store.party != null:
		var left: ApiResult = await leave()
		if not left.ok:
			return left
	_intent_keys.erase("accept:%s" % invitation.invitation_id)
	return await accept(invitation)


## Coroutine: `rejectPartyInvitation`.
func reject(invitation: InvitationModel) -> ApiResult:
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_REJECT % invitation.invitation_id),
			"rejectPartyInvitation")
	if result.ok or result.status == 404:
		store.remove_invitation(invitation.invitation_id)
	return result


## Coroutine: `leaveParty` (confirmed by the student, F102-AC-12).
func leave() -> ApiResult:
	if store.party == null:
		return ApiResult.failure(CODE_NOT_FOUND)
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_LEAVE % store.party.party_id), "leaveParty")
	if result.ok or result.status == 404:
		store.close(PartyStore.CLOSED_LEFT)
	return result


# ---------- intents (US2, leader controls checked again by the server) ----------

## Coroutine: Ready / not ready (F102-AC-08): WS command when the socket takes commands, else REST; the card changes
## only once the server confirmed. Returns true when confirmed.
func set_ready(ready: bool) -> bool:
	if store.party == null or is_offline():
		return false
	var me: String = store.self_id
	if realtime != null and realtime.can_write():
		var outcome: CommandResult = await realtime.send_command(CMD_READY, {"party_id": store.party.party_id, "ready": ready})
		if outcome.ok:
			return store.set_member_ready(me, ready)
		if outcome.code != CommandResult.CODE_OFFLINE and outcome.code != CommandResult.CODE_TIMEOUT:
			_notice_error(outcome.code)
			return false
	if store.party == null:
		return false
	var result: ApiResult = await _write(ApiRequest.make_put(PATH_READY % store.party.party_id, {"ready": ready}),
			"setPartyReady")
	return result.ok and store.set_member_ready(me, ready)


## Coroutine: the leader chooses `activity` (F102-AC-10) with the `state_version` on screen; a mismatch reloads the
## party and asks the leader to choose again (no automatic resend).
func select_activity(activity: EligibleActivityModel) -> bool:
	if store.party == null or activity == null or is_offline():
		return false
	store.titles[activity.activity_ref] = activity.title
	var expected: int = store.party.state_version
	if realtime != null and realtime.can_write():
		var outcome: CommandResult = await realtime.send_command(CMD_SELECT, {"party_id": store.party.party_id,
			"activity": {"kind": activity.activity_type, "id": activity.activity_ref}, "expected_version": expected})
		if outcome.ok:
			return true
		if outcome.code == CODE_WS_CONFLICT:
			_notice_error(CODE_VERSION_MISMATCH)
			refresh_party()
			return false
		if outcome.code != CommandResult.CODE_OFFLINE and outcome.code != CommandResult.CODE_TIMEOUT:
			_notice_error(outcome.code)
			return false
	if store.party == null:
		return false
	var body: Dictionary = {"activity_type": activity.activity_type, "activity_ref": activity.activity_ref,
		"expected_version": expected}
	var result: ApiResult = await _write(ApiRequest.make_put(PATH_ACTIVITY % store.party.party_id, body),
			"selectPartyActivity")
	if result.ok:
		store.apply_snapshot(PartyModel.from_rest(result.body()))
	elif result.code == CODE_VERSION_MISMATCH or result.status == 412:
		refresh_party()
	return result.ok


## Coroutine: the leader starts the session (`startPartySession`, F102-AC-09/11). `409 PARTY_ACTIVITY_NOT_ELIGIBLE`
## keeps the members the server named; `412 VERSION_MISMATCH` reloads the party and lets the leader press again.
func start_session() -> ApiResult:
	if store.party == null:
		return ApiResult.failure(CODE_NOT_FOUND)
	var req: ApiRequest = ApiRequest.make_post(PATH_START % store.party.party_id,
			{"expected_version": store.party.state_version})
	var result: ApiResult = await _write(req, "startPartySession", "start:%s:%d" % [store.party.party_id,
			store.party.state_version])
	if result.ok:
		store.start_session(result.body())
	elif result.code == CODE_VERSION_MISMATCH or result.status == 412:
		refresh_party()
	elif result.code == "PARTY_ACTIVITY_NOT_ELIGIBLE" and store.party != null:
		store.ineligible.clear()
		for raw: Variant in DataRead.arr_of(result.body(), "ineligible_members"):
			if raw is Dictionary:
				var e: Dictionary = raw as Dictionary
				store.ineligible.append({"student_id": DataRead.str_of(e, "student_id", DataRead.str_of(e, "user_id")),
					"reason_code": DataRead.str_of(e, "reason_code"), "requirement": DataRead.str_of(e, "requirement")})
		store.party_updated.emit(store.party)
	return result


func kick(student_id: String) -> ApiResult:
	if store.party == null:
		return ApiResult.failure(CODE_NOT_FOUND)
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_KICK % [store.party.party_id, student_id]),
			"kickPartyMember")
	if result.ok:
		store.remove_member(student_id)
	return result


func transfer_leader(student_id: String) -> ApiResult:
	if store.party == null:
		return ApiResult.failure(CODE_NOT_FOUND)
	var req: ApiRequest = ApiRequest.make_post(PATH_TRANSFER % store.party.party_id, {"new_leader_id": student_id})
	var result: ApiResult = await _write(req, "transferPartyLeader", "transfer:%s:%s" % [store.party.party_id, student_id])
	if result.ok:
		store.apply_snapshot(PartyModel.from_rest(result.body()))
	return result


func disband() -> ApiResult:
	if store.party == null:
		return ApiResult.failure(CODE_NOT_FOUND)
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_DISBAND % store.party.party_id), "disbandParty")
	if result.ok:
		store.close(PartyStore.CLOSED_DISBANDED)
	return result


## Coroutine: leader settings (`updatePartySettings`, merge patch): `visibility`, `invitation_policy`.
func update_settings(changes: Dictionary) -> ApiResult:
	if store.party == null:
		return ApiResult.failure(CODE_NOT_FOUND)
	var result: ApiResult = await _write(ApiRequest.make_patch(PATH_PARTY % store.party.party_id, changes),
			"updatePartySettings")
	if result.ok:
		store.apply_snapshot(PartyModel.from_rest(result.body()))
	return result


## Enters the room of the running session (after the 3-2-1 countdown, or again after a reconnect) through the
## SceneRouter. Returns OK, or an error when no session / scene is known.
func enter_session() -> Error:
	var scene_ref: String = String(store.session.get("scene_ref", ""))
	var room: String = String(store.session.get("room_id", ""))
	if String(store.session.get("room_type", "")) == ROOM_BOSS and not room.is_empty():
		if boss_router.is_valid():
			return boss_router.call(room, scene_ref) as Error
		return SceneRouter.enter_boss_encounter(room, scene_ref)
	if room.is_empty() or scene_ref.is_empty():
		return ERR_UNAVAILABLE
	if session_router.is_valid():
		return session_router.call(scene_ref, room) as Error
	return SceneRouter.enter_party_session(scene_ref, room)


# ---------- state for the screens ----------

## Writes are locked while the realtime connection comes back or the last REST call found no network.
func is_offline() -> bool:
	return offline or _reconnecting()


## The student may invite (leader, or every member with `invitation_policy = members`) and the party has room — a
## hint for the "+ Mời bạn" button only; the server checks again (R1).
func can_invite() -> bool:
	var p: PartyModel = store.party
	if p == null or p.is_full() or p.is_in_activity():
		return false
	return store.is_leader() or p.invitation_policy == "members"


## Words for the disabled Start button (F102-AC-9): "" when nothing is missing on screen.
func start_block_key() -> Dictionary:
	var p: PartyModel = store.party
	if p == null:
		return {}
	if not p.has_activity():
		return {"key": "ui.party.start_block.no_activity", "args": {}}
	var waiting: int = p.not_ready_count()
	if waiting == 1:
		return {"key": "ui.party.start_block.one_not_ready", "args": {}}
	if waiting > 1:
		return {"key": "ui.party.start_block.not_ready", "args": {"count": waiting}}
	if p.leader_reconnecting:
		return {"key": "ui.party.leader_reconnecting", "args": {}}
	return {}


## Friendly sentence key of a problem / command code (F102-AC-05).
static func error_key(code: String) -> String:
	return String(ERROR_KEYS.get(code, KEY_GENERIC_ERROR))


## Invitation popup allowed: popups on in Settings and not in an exam / challenge / activity (spec edge case).
func should_popup(_invitation: InvitationModel) -> bool:
	if quiet_check.is_valid() and bool(quiet_check.call()):
		return false
	if settings == null:
		return true
	var value: Variant = settings.get("notification_popups")
	return not (value is bool) or (value as bool)


func is_busy() -> bool:
	return _busy > 0


## Server time in ms (realtime clock when known) — invitation countdowns follow the server `expires_at`.
func now_ms() -> int:
	if realtime != null and realtime.server_offset_ms != 0:
		return realtime.server_now_ms()
	if api != null and api.clock != null:
		return int(api.clock.unix_time() * 1000.0)
	return int(Time.get_unix_time_from_system() * 1000.0)


## "Nhóm của {nickname}" in the student's language, within `PartyCreate.name` bounds.
func default_name() -> String:
	var nick: String = _self_nickname()
	return TranslationServer.translate(KEY_DEFAULT_NAME).format({"nickname": nick}).left(NAME_MAX_LENGTH)


## Realtime dispatcher handler for `party.*` (GAME_NETWORKING §4.2).
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	if store.self_id.is_empty():
		_refresh_self()
	store.apply_message(type, payload)


## Coroutine: resync after a failed resume or a reconnect (F102-AC-15).
func resync() -> void:
	await load_my_party()
	load_invitations()
	var p: PartyModel = store.party
	if p != null and p.is_in_activity() and store.me() != null and not p.room_id.is_empty():
		if String(store.session.get("room_id", "")) == p.room_id and SceneRouter.party_room_id != p.room_id:
			enter_session()


## Per-frame tick of the world (WorldRoot): at most once a second the invitations whose server `expires_at` passed
## leave the list (F102-AC-03).
func tick() -> void:
	var now: int = now_ms()
	if now - _last_expire_ms < EXPIRE_CHECK_MS:
		return
	_last_expire_ms = now
	store.expire_invitations(now)


## Logout / takeover / new session: nothing of the previous student is kept (F102-AC-19).
func clear() -> void:
	store.clear()
	friends.clear()
	friends_loaded = false
	invited.clear()
	eligible.clear()
	_intent_keys.clear()
	store.self_id = ""


# ---------- internals ----------

## Coroutine: one REST write. `intent` keeps the Idempotency-Key until the server answered (retry after a network
## failure = same key). Problems become a notice unless their code is in `silent`.
func _write(req: ApiRequest, operation_id: String, intent: String = "", silent: PackedStringArray = PackedStringArray()) -> ApiResult:
	if not intent.is_empty():
		if not _intent_keys.has(intent):
			_intent_keys[intent] = Uuid.v7()
		req.idempotency_key = String(_intent_keys[intent])
	_set_busy(1)
	var result: ApiResult = await send_request(req, operation_id)
	_set_busy(-1)
	if not intent.is_empty() and not result.network_error and not result.is_server_unavailable():
		_intent_keys.erase(intent)
	if not result.ok and not result.code in silent and not result.code in ACCESS_DENIED_CODES:
		_notice_error(result.code)
	return result


func _notice_error(code: String) -> void:
	notice_requested.emit(error_key(code), {})


func _set_busy(delta: int) -> void:
	var before: bool = _busy > 0
	_busy = maxi(0, _busy + delta)
	if before != (_busy > 0):
		busy_changed.emit(_busy > 0)


func _reconnecting() -> bool:
	return realtime != null and realtime.is_reconnecting()


func _self_id() -> String:
	var player: Dictionary = app_state.get("player") as Dictionary if app_state != null and app_state.get("player") is Dictionary else {}
	for path: Array in [["student", "student_id"], ["game_profile", "student_id"], ["user", "user_id"]]:
		var part: Dictionary = DataRead.dict_of(player, String(path[0]))
		var id: String = DataRead.str_of(part, String(path[1]))
		if Uuid.is_valid(id):
			return id
	return ""


## The signed-in student id from the bootstrap (kept when the bootstrap has none, e.g. in tests).
func _refresh_self() -> void:
	var id: String = _self_id()
	if not id.is_empty():
		store.self_id = id


func _self_nickname() -> String:
	var player: Dictionary = app_state.get("player") as Dictionary if app_state != null and app_state.get("player") is Dictionary else {}
	return DataRead.str_of(DataRead.dict_of(player, "game_profile"), "nickname")


func _on_connection_state_changed(_state: int) -> void:
	var now: bool = _reconnecting()
	connection_changed.emit(now or offline)
	if _was_reconnecting and not now and realtime != null and realtime.is_online():
		resync()
	_was_reconnecting = now


func _on_resync_needed() -> void:
	refresh_party()


func _on_party_updated(party: PartyModel) -> void:
	if app_state != null:
		app_state.set("party", party)


func _on_party_closed(reason: StringName) -> void:
	invited.clear()
	eligible.clear()
	match reason:
		PartyStore.CLOSED_DISBANDED:
			notice_requested.emit(KEY_DISBANDED, {})
		PartyStore.CLOSED_KICKED:
			notice_requested.emit(KEY_KICKED, {})


func _on_leader_changed(_from: String, to: String, reason: String) -> void:
	if reason == "manual" and to != store.self_id:
		return
	var m: PartyMemberModel = store.party.member(to) if store.party != null else null
	if m != null:
		notice_requested.emit(KEY_NEW_LEADER, {"nickname": m.nickname})


func _on_activity_changed(_party: PartyModel) -> void:
	notice_requested.emit(KEY_ACTIVITY_CHANGED, {})


func _on_invitation_received(invitation: InvitationModel) -> void:
	var tree: SceneTree = Engine.get_main_loop() as SceneTree
	var bus: Node = tree.root.get_node_or_null("EventBus") if tree != null else null
	if bus != null:
		bus.emit_signal("party_invitation_received", invitation)
