class_name ClubManageService
extends KeyedWriteService
## "Quản lý club" of the club owner (F112 US5, US6; F112-AC-18…AC-24; SOCIAL_SYSTEM §6.2–§6.3; backend F120). The only
## place of the client that calls `updateClub`, `updateClubMemberRole`, `createClubGoal`, `createClubEvent` (AGENTS R1):
## - `load_club(club_id)`: `getClub` (+ `version`), `listClubMembers` (every page), `listClubGoals` (`accepted_metrics`);
## - `save_club(form)`: only the fields that changed, `If-Match` = the `version` read last; the page follows the `Club`
##   the server returns; `412` → `getClub` again, the form is refilled and says "Thông tin club vừa thay đổi…";
## - `set_role(user_id, officer)`: `role` = `officer` / `member` (setting a value: safe to repeat); the row follows the
##   `ClubMember` returned; never on the owner's row (the screen has no action there);
## - `create_goal(form)` / `create_event(form, now)`: one `Idempotency-Key` per intent, kept while retried after a
##   network failure (one goal / event however many retries, AGENTS R10), then `listClubGoals` / `listClubEvents` again
##   (through ClubService so the club page shows them) and a 2-second toast (`saved`);
## - problems: `403` → the tools close (`tools_revoked`), the page is read again and says "Em không còn quyền quản lý club
##   này" (the role on the client only decides what is shown, AGENTS R7); `451 CHAT_CONTENT_BLOCKED` → the general
##   moderation sentence, nothing echoed, the form stays (AGENTS R15); `422` → a sentence next to the field;
##   `409 CLUB_NOT_APPROVED` → "Club đang chờ nhà trường duyệt"; `404` → the club is gone (ClubService closes the page).
## Telemetry: the code of a refused write only (`client_error`, scene `club_manage`) — never a name, a text or an id
## (AGENTS R14). RAM only; cleared on logout / takeover.

## The loaded club, members or a form state changed.
signal changed()
## A write succeeded: `what` = "club" / "role" / "goal" / "event" (the screen shows a 2-second toast).
signal saved(what: String)
## `403`: the student is not the owner any more — the tools close.
signal tools_revoked(club_id: String)

const PATH_CLUB: String = "/social/clubs/%s"
const PATH_MEMBERS: String = "/social/clubs/%s/members"
const PATH_MEMBER: String = "/social/clubs/%s/members/%s"
const PATH_GOALS: String = "/social/clubs/%s/goals"
const PATH_EVENTS: String = "/social/clubs/%s/events"
const CODE_BLOCKED: String = "CHAT_CONTENT_BLOCKED"
const CODE_NOT_APPROVED: String = "CLUB_NOT_APPROVED"
const KEY_CONFLICT: String = "ui.club.manage.conflict"
const KEY_BLOCKED: String = "ui.club.form.blocked"
const KEY_NOT_APPROVED: String = "ui.club.manage.not_approved"
const KEY_REVOKED: String = "ui.club.manage.revoked"
const KEY_FIELD: String = "ui.club.manage.field_invalid"
const KEY_OFFLINE: String = "ui.friends.offline"
const SCENE_CODE: String = "club_manage"

var clubs: ClubService = null
var club: ClubModel = null
var members: Array[ClubMemberModel] = []
var accepted_metrics: PackedStringArray = []
## The sentence of the last refused write ("" = none) and, for `422`, field → sentence key.
var form_error: String = ""
var field_errors: Dictionary = {}


func _init(rest: RestApiClient = null, telemetry_service: TelemetryService = null, club_service: ClubService = null) -> void:
	super(rest, telemetry_service)
	clubs = club_service


## The tools are shown only to the owner of an active club (F112-AC-18); the server still decides.
static func can_manage(c: ClubModel) -> bool:
	return c != null and c.is_owner() and c.is_active()


## Coroutine: club, members (every page) and the accepted goal metrics.
func load_club(club_id: String) -> ApiResult:
	if not Uuid.is_valid(club_id):
		return ApiResult.failure("VALIDATION_FAILED")
	_reset_errors()
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_CLUB % club_id), "getClub")
	if not result.ok:
		await _refused(result, club_id)
		return result
	club = ClubModel.from_dict(result.body())
	var pages: Dictionary = await send_all_pages(PATH_MEMBERS % club_id, {}, "listClubMembers")
	members = ClubMemberModel.list_from(pages["items"])
	var goals: ApiResult = await send_request(ApiRequest.make_get(PATH_GOALS % club_id), "listClubGoals")
	accepted_metrics.clear()
	if goals.ok:
		for m: Variant in DataRead.arr_of(goals.body(), "accepted_metrics"):
			if m is String:
				accepted_metrics.append(String(m))
	changed.emit()
	return result


## Coroutine: "Lưu" of "Sửa thông tin". Nothing changed → nothing sent (ok).
func save_club(form: Dictionary) -> ApiResult:
	_reset_errors()
	if club == null:
		return ApiResult.failure("NOT_FOUND")
	var changes: Dictionary = ClubFormOptions.club_changes(club, form)
	if changes.is_empty():
		return ApiResult.failure("NOTHING_CHANGED")
	var local: String = ClubFormOptions.check_club(changes)
	if not local.is_empty():
		form_error = local
		changed.emit()
		return ApiResult.failure("VALIDATION_FAILED")
	var req: ApiRequest = ApiRequest.make_patch(PATH_CLUB % club.club_id, changes)
	req.headers["If-Match"] = "\"%d\"" % club.version
	var result: ApiResult = await send_keyed(req, "updateClub", "")
	if result.ok:
		var next: ClubModel = ClubModel.from_dict(result.body())
		if next != null:
			club = next
		_refresh_page()
		saved.emit("club")
	elif result.status == 412:
		var id: String = club.club_id
		await load_club(id)
		form_error = KEY_CONFLICT
	else:
		await _refused(result, club.club_id)
	changed.emit()
	return result


## Coroutine: "Chọn làm phó nhóm" (`officer` true) / "Thôi làm phó nhóm" (false) of a member.
func set_role(user_id: String, officer: bool) -> ApiResult:
	_reset_errors()
	var row: ClubMemberModel = member(user_id)
	if club == null or row == null or row.role == ClubModel.ROLE_OWNER:
		return ApiResult.failure("VALIDATION_FAILED")
	var req: ApiRequest = ApiRequest.make_patch(PATH_MEMBER % [club.club_id, user_id],
			{"role": ClubModel.ROLE_OFFICER if officer else ClubModel.ROLE_MEMBER})
	var result: ApiResult = await send_keyed(req, "updateClubMemberRole", "")
	if result.ok:
		var next: ClubMemberModel = ClubMemberModel.from_dict(result.body())
		if next != null and next.user_id == user_id:
			row.role = next.role
		_refresh_page()
		saved.emit("role")
	else:
		await _refused(result, club.club_id)
	changed.emit()
	return result


## Coroutine: "Tạo mục tiêu" (weekly goal). The same form retried after a network failure keeps its key.
func create_goal(form: Dictionary) -> ApiResult:
	_reset_errors()
	if club == null:
		return ApiResult.failure("NOT_FOUND")
	var body: Dictionary = ClubFormOptions.goal_body(form, accepted_metrics)
	if body.is_empty():
		form_error = "ui.club.manage.goal.invalid"
		changed.emit()
		return ApiResult.failure("VALIDATION_FAILED")
	var intent: String = "goal:%s:%s" % [club.club_id, JSON.stringify(body).md5_text()]
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_GOALS % club.club_id, body), "createClubGoal", intent)
	await _after_create(result, "goal")
	return result


## Coroutine: "Tạo sự kiện" (club event); `now_unix` = the server clock (a start in the past is refused here).
func create_event(form: Dictionary, now_unix: int) -> ApiResult:
	_reset_errors()
	if club == null:
		return ApiResult.failure("NOT_FOUND")
	var local: String = ClubFormOptions.check_event(form, now_unix)
	if not local.is_empty():
		form_error = local
		changed.emit()
		return ApiResult.failure("VALIDATION_FAILED")
	var body: Dictionary = ClubFormOptions.event_body(form)
	var intent: String = "event:%s:%s" % [club.club_id, JSON.stringify(body).md5_text()]
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_EVENTS % club.club_id, body), "createClubEvent",
			intent)
	await _after_create(result, "event")
	return result


func member(user_id: String) -> ClubMemberModel:
	for m: ClubMemberModel in members:
		if m.user_id == user_id:
			return m
	return null


func clear() -> void:
	club = null
	members.clear()
	accepted_metrics.clear()
	_reset_errors()
	forget_keys()


func _after_create(result: ApiResult, what: String) -> void:
	if result.ok:
		_refresh_page()
		saved.emit(what)
	elif result.network_error or result.is_server_unavailable():
		form_error = KEY_OFFLINE
		_count(result)
	else:
		await _refused(result, club.club_id)
	changed.emit()


## The club page (F111) reads the club, goals and calendar again so it shows what was saved.
func _refresh_page() -> void:
	if clubs != null and club != null and clubs.store.page_id == club.club_id:
		clubs.open_club(club.club_id)


func _refused(result: ApiResult, club_id: String) -> void:
	_count(result)
	if result.network_error or result.is_server_unavailable():
		form_error = KEY_OFFLINE
		return
	match result.status:
		403:
			if result.code in ACCESS_DENIED_CODES:
				return
			form_error = KEY_REVOKED
			notice_requested.emit(KEY_REVOKED, {})
			tools_revoked.emit(club_id)
			if clubs != null:
				await clubs.open_club(club_id)
		404:
			tools_revoked.emit(club_id)
			if clubs != null:
				await clubs.open_club(club_id)
		409:
			form_error = KEY_NOT_APPROVED if result.code == CODE_NOT_APPROVED else error_key(result.code)
		422:
			form_error = KEY_FIELD
			if result.problem != null:
				for e: Dictionary in result.problem.errors:
					var field: String = DataRead.str_of(e, "field")
					if not field.is_empty():
						field_errors[field] = KEY_FIELD
		451:
			form_error = KEY_BLOCKED
		_:
			form_error = error_key(result.code)


func _count(result: ApiResult) -> void:
	if telemetry != null and not result.code.is_empty():
		telemetry.queue_client_error(result.code, SCENE_CODE)
	Log.info("club_manage_refused", {"status": result.status, "code": result.code})


func _reset_errors() -> void:
	form_error = ""
	field_errors.clear()
