class_name TournamentService
extends GameApiService
## Tournaments and grade challenges of the student (F109 US5; COMPETITION_SYSTEM §10, F107). The server lists only
## what the student belongs to (grade ± students − exclusions, ADR-0095); the client never filters by grade.
## Register / withdraw / check-in carry one `Idempotency-Key` per intent where the contract takes one (R10,
## F109-AC-24); the check-in button follows the server window only. `tournament.match.scheduled` (10 / 1 minute notice)
## raises a toast that opens the tournament tab (F109-AC-26; a `tournament_update` notification uses the F123 toast and
## NotificationService.KIND_SCREENS);
## a tournament match is entered through the ready check like any match (MatchmakingService).

signal tournaments_changed()
signal detail_changed(tournament_id: String)
signal bracket_changed(tournament_id: String)
signal challenges_changed()
## A friendly line for the screens (`key` + args).
signal notice_requested(key: String, args: Dictionary)
## Toast: `key` + args, and the tournament to open.
signal toast_requested(key: String, args: Dictionary, tournament_id: String)
signal busy_changed(busy: bool)

const PREFIX: String = "tournament"
const PATH_LIST: String = "/competition/tournaments"
const PATH_ONE: String = "/competition/tournaments/%s"
const PATH_BRACKET: String = "/competition/tournaments/%s/bracket"
const PATH_ENTRIES: String = "/competition/tournaments/%s/entries"
const PATH_ENTRY: String = "/competition/tournaments/%s/entries/%s"
const PATH_CHECK_IN: String = "/competition/tournaments/%s/entries/%s:check-in"
const PATH_CHALLENGES: String = "/competition/challenges"
const PATH_JOIN: String = "/competition/challenges/%s:join"
const MSG_SCHEDULED: String = "tournament.match.scheduled"
const MSG_BRACKET: String = "tournament.bracket.updated"

var realtime: GameRealtimeClient = null
var tournaments: Array[TournamentModel] = []
var tournaments_loaded: bool = false
var page_cursor: String = ""
## tournament_id → TournamentModel (detail with `my_entry`).
var details: Dictionary = {}
## tournament_id → BracketModel.
var brackets: Dictionary = {}
var challenges: Array[GradeChallengeModel] = []
var _intent_keys: Dictionary = {}
var _busy: int = 0


func _init(rest: RestApiClient = null, realtime_client: GameRealtimeClient = null, telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	realtime = realtime_client
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)


## Coroutine: `listTournaments` (cursor; `more` appends the next page).
func load_list(more: bool = false) -> ApiResult:
	var query: Dictionary = {"limit": GameApiService.PAGE_LIMIT}
	if more and not page_cursor.is_empty():
		query["cursor"] = page_cursor
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_LIST, query), "listTournaments")
	if result.ok:
		var list: Array[TournamentModel] = TournamentModel.list_from(result.body().get("data"))
		if more:
			tournaments.append_array(list)
		else:
			tournaments = list
		page_cursor = GameApiService.next_cursor(result.body())
		tournaments_loaded = true
		tournaments_changed.emit()
	return result


## Coroutine: `getTournament` (with the student's `my_entry`). 404 = out of scope (F109-AC-29).
func load_detail(tournament_id: String) -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_ONE % tournament_id), "getTournament")
	if result.ok:
		var t: TournamentModel = TournamentModel.from_dict(result.body())
		if t != null:
			details[tournament_id] = t
			_replace_in_list(t)
			detail_changed.emit(tournament_id)
	elif result.status == 404:
		details.erase(tournament_id)
		notice_requested.emit(CompetitionText.error_key("NOT_FOUND"), {})
	return result


## Coroutine: `getTournamentBracket`.
func load_bracket(tournament_id: String) -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_BRACKET % tournament_id), "getTournamentBracket")
	if result.ok:
		var b: BracketModel = BracketModel.from_dict(result.body())
		if b != null:
			brackets[tournament_id] = b
			bracket_changed.emit(tournament_id)
	return result


## Coroutine: `registerTournamentEntry` (F109-AC-24).
func register(tournament_id: String, party_id: String = "") -> ApiResult:
	var body: Dictionary = {}
	if Uuid.is_valid(party_id):
		body["party_id"] = party_id
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_ENTRIES % tournament_id, body), "registerTournamentEntry",
			"register:%s:%s" % [tournament_id, party_id])
	# Already registered (409 CONFLICT), full (409 TOURNAMENT_FULL) or outside the window (422): the detail is read
	# again so the buttons follow the server state.
	if result.ok or result.status == 409 or result.status == 422:
		await load_detail(tournament_id)
	return result


## Coroutine: `withdrawTournamentEntry` (a soft withdraw before the bracket is locked).
func withdraw(t: TournamentModel) -> ApiResult:
	if t.my_entry == null:
		return ApiResult.failure("NOT_FOUND")
	var req: ApiRequest = ApiRequest.make_get(PATH_ENTRY % [t.tournament_id, t.my_entry.entry_id])
	req.method = "DELETE"
	var result: ApiResult = await _write(req, "withdrawTournamentEntry")
	await load_detail(t.tournament_id)
	return result


## Coroutine: `checkInTournamentEntry` — only offered inside the server window (F109-AC-24).
func check_in(t: TournamentModel) -> ApiResult:
	if t.my_entry == null:
		return ApiResult.failure("NOT_FOUND")
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_CHECK_IN % [t.tournament_id, t.my_entry.entry_id]),
			"checkInTournamentEntry")
	if result.ok:
		var e: TournamentEntryModel = TournamentEntryModel.from_dict(result.body())
		var held: TournamentModel = details.get(t.tournament_id, t) as TournamentModel
		if e != null:
			held.my_entry = e
			detail_changed.emit(t.tournament_id)
	elif result.status == 422:
		await load_detail(t.tournament_id)
	return result


## Coroutine: `listMyChallenges` (grade challenges of the school the student belongs to).
func load_challenges() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_CHALLENGES, {"limit": GameApiService.PAGE_LIMIT}),
			"listMyChallenges")
	if result.ok:
		challenges = GradeChallengeModel.list_from(result.body().get("data"))
		challenges_changed.emit()
	return result


## Coroutine: `joinChallenge` → the Timed Quiz room (`match_id`) the battle screen enters with `battle.state.request`.
func join_challenge(challenge_id: String) -> ApiResult:
	return await _write(ApiRequest.make_post(PATH_JOIN % challenge_id), "joinChallenge", "join:%s" % challenge_id)


func tournament(tournament_id: String) -> TournamentModel:
	if details.has(tournament_id):
		return details[tournament_id] as TournamentModel
	for t: TournamentModel in tournaments:
		if t.tournament_id == tournament_id:
			return t
	return null


## Realtime handler for `tournament.*` (F109-AC-26): a match notice → toast; a bracket change → the bracket again.
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	var id: String = DataRead.str_of(payload, "tournament_id")
	if not Uuid.is_valid(id):
		return
	match type:
		MSG_SCHEDULED:
			var minutes: int = DataRead.int_of(payload, "notice_minutes", 10)
			var t: TournamentModel = tournament(id)
			toast_requested.emit("ui.tournament.toast.match_soon", {"minutes": minutes,
				"name": t.name if t != null else TranslationServer.translate("ui.tournament.title")}, id)
		MSG_BRACKET:
			if brackets.has(id):
				load_bracket(id)



func now_ms() -> int:
	if realtime != null and realtime.server_offset_ms != 0:
		return realtime.server_now_ms()
	if api != null and api.clock != null:
		return int(api.clock.unix_time() * 1000.0)
	return int(Time.get_unix_time_from_system() * 1000.0)


func is_busy() -> bool:
	return _busy > 0


func clear() -> void:
	tournaments.clear()
	tournaments_loaded = false
	page_cursor = ""
	details.clear()
	brackets.clear()
	challenges.clear()
	_intent_keys.clear()


func _replace_in_list(t: TournamentModel) -> void:
	for i: int in tournaments.size():
		if tournaments[i].tournament_id == t.tournament_id:
			tournaments[i] = t
			return


func _write(req: ApiRequest, operation_id: String, intent: String = "") -> ApiResult:
	if not intent.is_empty():
		if not _intent_keys.has(intent):
			_intent_keys[intent] = Uuid.v7()
		req.idempotency_key = String(_intent_keys[intent])
	_set_busy(1)
	var result: ApiResult = await send_request(req, operation_id)
	_set_busy(-1)
	if not intent.is_empty() and not result.network_error and not result.is_server_unavailable():
		_intent_keys.erase(intent)
	if not result.ok and not result.code in ACCESS_DENIED_CODES:
		notice_requested.emit(CompetitionText.tournament_error_key(result.code), {})
	return result


func _set_busy(delta: int) -> void:
	var before: bool = _busy > 0
	_busy = maxi(0, _busy + delta)
	if before != (_busy > 0):
		busy_changed.emit(_busy > 0)
