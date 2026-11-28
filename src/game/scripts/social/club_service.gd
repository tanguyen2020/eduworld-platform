class_name ClubService
extends KeyedWriteService
## Clubs of the signed-in student (F111 US5, US6; SOCIAL_SYSTEM §6, ADR-0050, ADR-0054; backend F120). The only place of
## the client that calls `/social/clubs*` and `/social/club-*` (AGENTS R1); the club tab reads `store` and calls:
## - `load_lists(band)` — `listClubs` `scope = mine`, `scope = discoverable` (never for lớp 1–3, STUDENT_GAME_UX_SOCIAL §6)
##   and `listMyClubInvitations` (pending) (F111-AC-19); a legacy `study_group` is dropped by ClubModel;
## - `open_club(club_id)` — `getClub` + `listClubMembers` + `listClubGoals` + `listClubEvents` (+ `listClubJoinRequests`
##   for an owner / officer); `404` closes the page with a general sentence (F111-AC-20, AC-31);
## - `request_join`, `accept_invitation` (Idempotency-Key kept when retried), `decline_invitation` (F111-AC-21);
## - `create(form)` — checked by ClubForm first (2–64 / ≤ 500 characters, student types only); `451` → a general
##   moderation sentence (nothing echoed, AGENTS R15), `403 PERMISSION_DENIED` → "Trường em chưa mở tạo club" and the
##   button hidden for the session (`creation_closed`) (F111-AC-22, AC-23);
## - `leave` (F111-AC-24), `accept_request` / `reject_request` (owner / officer; `403` → the page is read again — the role on
##   the client only decides what is shown, F111-AC-25), `invite(ids)` (≤ 50 friends; who was not invited reads "Chưa mời
##   được" with no reason, F111-AC-26).
## Problems become `ui.error.<CODE>` sentences. Ids only in logs, never a club name or a nickname (R14); RAM only.

## The club page is gone (`404`) or was left: the screen goes back to the list.
signal page_closed(club_id: String)
## A club the student created came back `active`: the screen opens its page.
signal club_created(club: ClubModel)

const PATH_CLUBS: String = "/social/clubs"
const PATH_CLUB: String = "/social/clubs/%s"
const PATH_MEMBERS: String = "/social/clubs/%s/members"
const PATH_GOALS: String = "/social/clubs/%s/goals"
const PATH_EVENTS: String = "/social/clubs/%s/events"
const PATH_REQUESTS: String = "/social/clubs/%s/join-requests"
const PATH_ACCEPT_REQUEST: String = "/social/club-join-requests/%s:accept"
const PATH_REJECT_REQUEST: String = "/social/club-join-requests/%s:reject"
const PATH_INVITE: String = "/social/clubs/%s/invitations"
const PATH_INVITATIONS: String = "/social/club-invitations"
const PATH_ACCEPT_INVITATION: String = "/social/club-invitations/%s:accept"
const PATH_DECLINE_INVITATION: String = "/social/club-invitations/%s:decline"
const PATH_LEAVE: String = "/social/clubs/%s:leave"
const MAX_INVITEES: int = 50
const CODE_BLOCKED: String = "CHAT_CONTENT_BLOCKED"
const CODE_DENIED: String = "PERMISSION_DENIED"

var store: ClubStore = ClubStore.new()
## `403 PERMISSION_DENIED` on `createClub`: the school closed club creation — "Tạo club" stays hidden this session.
var creation_closed: bool = false
## `403` on a join request action: the student is not owner / officer any more — the "Yêu cầu" section stays hidden.
var requests_denied: bool = false


func _init(rest: RestApiClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)


## Coroutine: "Club của em", "Club đề xuất" (not for lớp 1–3) and the pending invitations.
func load_lists(band: String) -> ApiResult:
	var mine: Dictionary = await send_all_pages(PATH_CLUBS, {"scope": "mine"}, "listClubs")
	if not bool(mine["ok"]):
		return mine["result"] as ApiResult
	var discover_items: Array = []
	if band != "g1_3":
		var discover: Dictionary = await send_all_pages(PATH_CLUBS, {"scope": "discoverable"}, "listClubs")
		discover_items = discover["items"] as Array
	var invites: Dictionary = await send_all_pages(PATH_INVITATIONS, {"status": "pending"}, "listMyClubInvitations")
	store.set_lists(ClubModel.list_from(mine["items"]), ClubModel.list_from(discover_items),
			ClubInvitationModel.list_from(invites["items"]))
	return mine["result"] as ApiResult


## Coroutine: the page of `club_id` (club, members, goals, calendar; requests for an owner / officer).
func open_club(club_id: String) -> ApiResult:
	if not Uuid.is_valid(club_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_CLUB % club_id), "getClub")
	if not result.ok:
		if result.status == 404 or result.status == 403:
			await _gone(club_id)
		else:
			notice_for(result)
		return result
	var club: ClubModel = ClubModel.from_dict(result.body())
	if club == null:
		await _gone(club_id)
		return ApiResult.failure("NOT_FOUND")
	requests_denied = false
	store.open_page(club)
	var members: Dictionary = await send_all_pages(PATH_MEMBERS % club_id, {}, "listClubMembers")
	var goals: ApiResult = await send_request(ApiRequest.make_get(PATH_GOALS % club_id), "listClubGoals")
	var events: Dictionary = await send_all_pages(PATH_EVENTS % club_id, {}, "listClubEvents")
	if store.page_id != club_id:
		return result
	store.set_page_parts(ClubMemberModel.list_from(members["items"]),
			ClubGoalModel.list_from(goals.body().get("data") if goals.ok else []), ClubEventModel.list_from(events["items"]))
	if club.is_officer():
		await list_requests(club_id)
	return result


## Coroutine: `listClubJoinRequests` (owner / officer only — the server says `403` otherwise).
func list_requests(club_id: String) -> ApiResult:
	var pages: Dictionary = await send_all_pages(PATH_REQUESTS % club_id, {}, "listClubJoinRequests")
	var result: ApiResult = pages["result"] as ApiResult
	if bool(pages["ok"]):
		store.set_requests(ClubJoinRequestModel.list_from(pages["items"]))
	elif result != null and result.status == 403:
		requests_denied = true
		store.set_requests([] as Array[ClubJoinRequestModel])
	return result


## Coroutine: "Xin vào" — `requestJoinClub`.
func request_join(club_id: String) -> ApiResult:
	if not Uuid.is_valid(club_id):
		return ApiResult.failure("VALIDATION_FAILED")
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_REQUESTS % club_id), "requestJoinClub",
			"join:%s" % club_id)
	if result.ok:
		store.mark_requested(club_id)
		notice_requested.emit("ui.club.request_sent", {})
	else:
		notice_for(result)
	return result


## Coroutine: "Đồng ý" of an invitation — `acceptClubInvitation`.
func accept_invitation(inv: ClubInvitationModel) -> ApiResult:
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_ACCEPT_INVITATION % inv.invitation_id),
			"acceptClubInvitation", "accept:%s" % inv.invitation_id)
	if result.ok:
		store.remove_invitation(inv.invitation_id)
		notice_requested.emit("ui.club.joined", {"club": inv.club_name})
		await load_lists(AppState.grade_band)
	elif not result.network_error and not result.is_server_unavailable():
		notice_for(result)
		if result.status == 404 or result.status == 410:
			store.remove_invitation(inv.invitation_id)
	else:
		notice_for(result)
	return result


## Coroutine: "Từ chối" of an invitation — `declineClubInvitation`.
func decline_invitation(inv: ClubInvitationModel) -> ApiResult:
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_DECLINE_INVITATION % inv.invitation_id),
			"declineClubInvitation", "")
	if result.ok or result.status == 404:
		store.remove_invitation(inv.invitation_id)
	else:
		notice_for(result)
	return result


## Coroutine: "Tạo club" — `createClub` after the form check. Returns the result (the form stays open on a problem).
func create(form: Dictionary) -> ApiResult:
	var problem: String = ClubForm.validate(form)
	if not problem.is_empty():
		return ApiResult.failure("VALIDATION_FAILED")
	var body: Dictionary = ClubForm.body(form)
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_CLUBS, body), "createClub",
			"create:%s:%s" % [String(body["club_type"]), String(body["name"]).md5_text()])
	if result.ok:
		var club: ClubModel = ClubModel.from_dict(result.body())
		await load_lists(AppState.grade_band)
		if club != null and club.is_active():
			club_created.emit(club)
		else:
			notice_requested.emit("ui.club.created_pending", {})
		return result
	match result.code:
		CODE_BLOCKED:
			pass # The form says the general moderation sentence itself (nothing echoed, AGENTS R15).
		CODE_DENIED:
			creation_closed = true
			notice_requested.emit("ui.club.not_enabled", {})
			store.clubs_changed.emit()
		_:
			notice_for(result)
	return result


## Coroutine: "Rời club" (after the one confirmation) — `leaveClub`.
func leave(club_id: String) -> ApiResult:
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_LEAVE % club_id), "leaveClub", "")
	if result.ok:
		notice_requested.emit("ui.club.left", {})
		store.close_page()
		page_closed.emit(club_id)
		await load_lists(AppState.grade_band)
	elif result.status == 404:
		await _gone(club_id)
	else:
		notice_for(result)
	return result


## Coroutine: "Chấp nhận" of a join request (owner / officer) — `acceptClubJoinRequest`.
func accept_request(req: ClubJoinRequestModel) -> ApiResult:
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_ACCEPT_REQUEST % req.request_id),
			"acceptClubJoinRequest", "accept_request:%s" % req.request_id)
	return await _after_request(req, result)


## Coroutine: "Từ chối" of a join request — `rejectClubJoinRequest`.
func reject_request(req: ClubJoinRequestModel) -> ApiResult:
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_REJECT_REQUEST % req.request_id),
			"rejectClubJoinRequest", "")
	return await _after_request(req, result)


## Coroutine: "Mời bạn" — `inviteToClub` with ≤ 50 friend ids. Returns `{ok_ids, failed_ids}`: a friend missing from the
## server answer (scope, block — the reason is never told) or a whole refusal reads "Chưa mời được".
func invite(club_id: String, ids: PackedStringArray) -> Dictionary:
	var picked: PackedStringArray = []
	for id: String in ids:
		if Uuid.is_valid(id) and not id in picked and picked.size() < MAX_INVITEES:
			picked.append(id)
	var out: Dictionary = {"ok_ids": PackedStringArray(), "failed_ids": PackedStringArray()}
	if picked.is_empty():
		return out
	var sorted: PackedStringArray = picked.duplicate()
	sorted.sort()
	var result: ApiResult = await send_keyed(ApiRequest.make_post(PATH_INVITE % club_id, {"invitee_ids": Array(picked)}),
			"inviteToClub", "invite:%s:%s" % [club_id, ",".join(sorted).md5_text()])
	var invited: Dictionary = {}
	if result.ok:
		for inv: ClubInvitationModel in ClubInvitationModel.list_from(result.body().get("data")):
			invited[inv.invitee_id] = true
	elif result.network_error or result.is_server_unavailable() or result.code == "RATE_LIMITED":
		notice_for(result)
	var ok_ids: PackedStringArray = []
	var failed_ids: PackedStringArray = []
	for id: String in picked:
		if invited.has(id):
			ok_ids.append(id)
		else:
			failed_ids.append(id)
	return {"ok_ids": ok_ids, "failed_ids": failed_ids}


func clear() -> void:
	store.clear()
	forget_keys()
	creation_closed = false
	requests_denied = false


func _after_request(req: ClubJoinRequestModel, result: ApiResult) -> ApiResult:
	if result.ok or result.status == 404:
		store.remove_request(req.request_id)
		return result
	if result.status == 403:
		requests_denied = true
		await open_club(store.page_id)
		return result
	notice_for(result)
	return result


func _gone(club_id: String) -> void:
	if store.page_id == club_id or store.page_id.is_empty():
		store.close_page()
	notice_requested.emit("ui.club.gone", {})
	page_closed.emit(club_id)
	await load_lists(AppState.grade_band)
