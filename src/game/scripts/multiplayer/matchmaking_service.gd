class_name MatchmakingService
extends GameApiService
## Entry into a match (F109 US1, US6; MATCHMAKING §5–§9, GAME_NETWORKING §7; GODOT_ARCHITECTURE §4): the modes the
## server opened, the subject / topic names of the picker, the queue ticket, the ready check, the practice robot and
## friend duels. The only place of the client that calls these operations (R1): the arena screen, the HUD queue bar
## and the ready-check dialog read its state and call its intents.
## - Writes (`createMatchmakingTicket`, `startBotMatch`, `createDuel`, `acceptDuel`) carry one `Idempotency-Key` per
##   intent, kept while the same intent is retried after a network failure (R10, F109-AC-02, AC-27, AC-28).
## - Cancel / accept / decline go over the realtime connection when it takes commands, else over REST (fallbacks of
##   the contract); the state changes only with what the server confirmed.
## - `matchmaking.*` messages update the ticket and the proposal; `matchmaking.match.assigned` (or a duel / robot /
##   challenge match) raises `match_assigned` — the world opens the battle screen.
## - `409 MM_TICKET_ALREADY_ACTIVE` reopens the known ticket (`getMatchmakingTicket`); a timeout suggests another
##   mode or the unranked robot (F109-AC-04). After a reconnect the held ticket is read again; a lost ticket
##   (FR-MM-006) ends the queue with "Hàng chờ đã kết thúc, em tìm lại nhé".
## The client never decides a mode, a subject or an opponent by grade: the server does (R1, R5). Nicknames and ids of
## other students are never logged (R14); nothing is written to disk.

signal modes_changed()
signal subjects_changed()
signal ticket_changed(ticket: MatchmakingTicketModel)
## The queue ended by itself: `reason` "timeout" (suggest another mode / robot), "lost" (ticket gone), "cancelled".
signal queue_ended(reason: String)
signal proposal_changed(proposal: MatchProposalModel)
## A proposal closed: `requeued` true when the student is back in the queue (someone else declined).
signal proposal_closed(requeued: bool)
signal match_assigned(match_id: String, mode: String)
signal notice_requested(key: String, args: Dictionary)
signal busy_changed(busy: bool)

const PREFIX: String = "matchmaking"
const PATH_MODES: String = "/competition/modes"
const PATH_TICKETS: String = "/competition/matchmaking/tickets"
const PATH_TICKET: String = "/competition/matchmaking/tickets/%s"
const PATH_BOT: String = "/competition/matchmaking/tickets/%s:bot-match"
const PATH_ACCEPT: String = "/competition/matches/%s/proposal:accept"
const PATH_DECLINE: String = "/competition/matches/%s/proposal:decline"
const PATH_DUELS: String = "/competition/duels"
const PATH_DUEL_ACCEPT: String = "/competition/duels/%s:accept"
const PATH_DUEL_DECLINE: String = "/competition/duels/%s:decline"
const PATH_SUBJECTS: String = "/learning/subjects"
const PATH_TOPICS: String = "/learning/topics"
const CMD_CANCEL: String = "matchmaking.ticket.cancel"
const CMD_ACCEPT: String = "matchmaking.proposal.accept"
const CMD_DECLINE: String = "matchmaking.proposal.decline"
const MSG_TICKET: String = "matchmaking.ticket.updated"
const MSG_PROPOSAL: String = "matchmaking.proposal.created"
const MSG_PROPOSAL_UPDATED: String = "matchmaking.proposal.updated"
const MSG_PROPOSAL_CANCELLED: String = "matchmaking.proposal.cancelled"
const MSG_ASSIGNED: String = "matchmaking.match.assigned"
const CODE_ACTIVE: String = "MM_TICKET_ALREADY_ACTIVE"
const KEY_QUEUE_LOST: String = "ui.arena.queue.lost"
const KEY_ALREADY_QUEUED: String = "ui.arena.error.MM_TICKET_ALREADY_ACTIVE"

var realtime: GameRealtimeClient = null
var app_state: Node = null
var modes: Array[ArenaModeModel] = []
var modes_loaded: bool = false
## subject_id → `{vi, en}` name (from `listSubjects`; an unknown id shows "Môn học").
var subject_names: Dictionary = {}
## subject_id → Array of `{topic_id, name}` (from `listTopics`, top level only).
var topics: Dictionary = {}
var ticket: MatchmakingTicketModel = null
var proposal: MatchProposalModel = null
## The last match the server assigned (the battle screen enters it).
var assigned_match_id: String = ""
var assigned_mode: String = ""
## The last search of the student `{mode, subject_id, topic_id}` ("Chơi lại", "Tìm lại").
var last_search: Dictionary = {}
var _intent_keys: Dictionary = {}
var _busy: int = 0
var _was_reconnecting: bool = false


func _init(rest: RestApiClient = null, realtime_client: GameRealtimeClient = null, app_state_node: Node = null,
		telemetry_service: TelemetryService = null) -> void:
	super(rest, telemetry_service)
	realtime = realtime_client
	app_state = app_state_node
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)
		realtime.connection_state_changed.connect(_on_connection_state_changed)


# ---------- reads ----------

## Coroutine: `listCompetitionModes` — the modes of the student (already filtered by the server).
func load_modes() -> ApiResult:
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_MODES), "listCompetitionModes")
	if result.ok:
		modes = ArenaModeModel.list_from(result.body().get("data"))
		modes_loaded = true
		modes_changed.emit()
	return result


## Coroutine: subject names of the picker (`listSubjects` of the student's grade; names only, R1).
func load_subjects() -> ApiResult:
	var query: Dictionary = {}
	var grade: int = CompetitionPlayer.grade_level(app_state)
	if grade > 0:
		query["grade_level"] = grade
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_SUBJECTS, query), "listSubjects")
	if result.ok:
		for raw: Variant in DataRead.arr_of(result.body(), "items"):
			if raw is Dictionary and Uuid.is_valid(DataRead.str_of(raw as Dictionary, "subject_id")):
				subject_names[DataRead.str_of(raw as Dictionary, "subject_id")] = DataRead.text_of(raw as Dictionary, "name")
		subjects_changed.emit()
	return result


## Coroutine: the topics of `subject_id` for the topic queue (`listTopics`, top level), cached for the session.
func load_topics(subject_id: String) -> ApiResult:
	if topics.has(subject_id):
		return ApiResult.new()
	var query: Dictionary = {"subject_id": subject_id}
	var grade: int = CompetitionPlayer.grade_level(app_state)
	if grade > 0:
		query["grade_level"] = grade
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_TOPICS, query), "listTopics")
	if result.ok:
		var list: Array[Dictionary] = []
		for raw: Variant in DataRead.arr_of(result.body(), "data"):
			if raw is Dictionary and DataRead.str_of(raw as Dictionary, "parent_topic_id").is_empty():
				var id: String = DataRead.str_of(raw as Dictionary, "topic_id")
				if Uuid.is_valid(id):
					list.append({"topic_id": id, "name": DataRead.text_of(raw as Dictionary, "name")})
		topics[subject_id] = list
		subjects_changed.emit()
	return result


func subject_name(subject_id: String) -> String:
	var name: String = L10n.pick(subject_names.get(subject_id, {}) as Dictionary)
	return name if not name.is_empty() else TranslationServer.translate("ui.arena.subject.unknown")


func mode(mode_code: String) -> ArenaModeModel:
	for m: ArenaModeModel in modes:
		if m.mode == mode_code:
			return m
	return null


## Subjects of `mode_code` (server list; empty = every subject the student has).
func subjects_of(mode_code: String) -> PackedStringArray:
	var m: ArenaModeModel = mode(mode_code)
	if m != null and not m.subjects.is_empty():
		return m.subjects
	return PackedStringArray(subject_names.keys())


# ---------- queue (US1) ----------

## Coroutine: `createMatchmakingTicket` (F109-AC-02). The same key while this intent is retried.
func find_match(mode_code: String, subject_id: String, topic_id: String = "", party_id: String = "") -> ApiResult:
	var body: Dictionary = {"mode": mode_code, "subject_id": subject_id}
	var m: ArenaModeModel = mode(mode_code)
	if Uuid.is_valid(topic_id):
		body["topic_id"] = topic_id
		if m != null and not m.matchmaking_type_for(topic_id).is_empty():
			body["matchmaking_type"] = m.matchmaking_type_for(topic_id)
	if Uuid.is_valid(party_id):
		body["party_id"] = party_id
	last_search = {"mode": mode_code, "subject_id": subject_id, "topic_id": topic_id}
	var intent: String = "ticket:%s:%s:%s:%s" % [mode_code, subject_id, topic_id, party_id]
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_TICKETS, body), "createMatchmakingTicket", intent,
			PackedStringArray([CODE_ACTIVE]))
	if result.ok:
		_set_ticket(MatchmakingTicketModel.from_dict(result.body()))
	elif result.code == CODE_ACTIVE:
		# F109-AC-04: the ticket of this student is already queued — show it (the problem carries no ticket id: the
		# one held in RAM is read again; without one the student is told and the bar waits for the next update).
		if ticket != null:
			await refresh_ticket()
		else:
			notice_requested.emit(KEY_ALREADY_QUEUED, {})
	return result


## Coroutine: `getMatchmakingTicket` of the held ticket (reconnect, MM_TICKET_ALREADY_ACTIVE). 404 = lost.
func refresh_ticket() -> ApiResult:
	if ticket == null:
		return ApiResult.failure("NOT_FOUND")
	var result: ApiResult = await send_request(ApiRequest.make_get(PATH_TICKET % ticket.ticket_id), "getMatchmakingTicket")
	if result.ok:
		_apply_ticket(MatchmakingTicketModel.from_dict(result.body()))
	elif result.status == 404:
		_end_queue("lost")
		notice_requested.emit(KEY_QUEUE_LOST, {})
	return result


## Coroutine: leave the queue — `matchmaking.ticket.cancel`, fallback `cancelMatchmakingTicket` (F109-AC-02).
func cancel() -> bool:
	if ticket == null:
		return false
	var id: String = ticket.ticket_id
	if realtime != null and realtime.can_write():
		var outcome: CommandResult = await realtime.send_command(CMD_CANCEL, {"ticket_id": id})
		if outcome.ok:
			_end_queue("cancelled")
			return true
		if outcome.code != CommandResult.CODE_OFFLINE and outcome.code != CommandResult.CODE_TIMEOUT:
			if outcome.code == "NOT_FOUND":
				_end_queue("cancelled")
				return true
			_notice(outcome.code)
			return false
	var req: ApiRequest = ApiRequest.make_get(PATH_TICKET % id)
	req.method = "DELETE"
	var result: ApiResult = await _write(req, "cancelMatchmakingTicket")
	if result.ok or result.status == 404:
		_end_queue("cancelled")
	return result.ok or result.status == 404


## Coroutine: "Luyện với Robot" (`startBotMatch`, unranked, labelled) — offered by the server after `mm_bot_after`.
func start_bot_match() -> ApiResult:
	if ticket == null:
		return ApiResult.failure("NOT_FOUND")
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_BOT % ticket.ticket_id), "startBotMatch",
			"bot:%s" % ticket.ticket_id)
	if result.ok:
		var match_id: String = DataRead.str_of(result.body(), "match_id")
		_end_queue("bot")
		_assign(match_id, DataRead.str_of(result.body(), "mode", "battle_1v1"))
	return result


# ---------- ready check (US1) ----------

## Coroutine: "Vào trận" — `matchmaking.proposal.accept`, fallback `acceptMatchProposal` (F109-AC-03).
func accept_proposal() -> bool:
	return await _answer_proposal(true)


## Coroutine: "Để lần sau" — `matchmaking.proposal.decline`, fallback `declineMatchProposal`.
func decline_proposal() -> bool:
	return await _answer_proposal(false)


func _answer_proposal(accept: bool) -> bool:
	if proposal == null:
		return false
	var id: String = proposal.proposal_id
	if realtime != null and realtime.can_write():
		var outcome: CommandResult = await realtime.send_command(CMD_ACCEPT if accept else CMD_DECLINE, {"proposal_id": id})
		if outcome.ok:
			return _proposal_answered(id, accept)
		if outcome.code != CommandResult.CODE_OFFLINE and outcome.code != CommandResult.CODE_TIMEOUT:
			_notice(outcome.code)
			return false
	var path: String = (PATH_ACCEPT if accept else PATH_DECLINE) % id
	var result: ApiResult = await _write(ApiRequest.make_post(path), "acceptMatchProposal" if accept else "declineMatchProposal")
	return result.ok and _proposal_answered(id, accept)


func _proposal_answered(id: String, accept: bool) -> bool:
	if proposal == null or proposal.proposal_id != id:
		return false
	if accept:
		proposal.accepted = true
		proposal_changed.emit(proposal)
	else:
		proposal = null
		proposal_closed.emit(false)
		_end_queue("cancelled")
	return true


# ---------- friend duel (US6) ----------

## Coroutine: `createDuel` (F109-AC-27); unranked, the friend has 60 s to answer.
func challenge_friend(opponent_id: String, subject_id: String, topic_id: String = "") -> ApiResult:
	var body: Dictionary = {"opponent_id": opponent_id, "subject_id": subject_id}
	if Uuid.is_valid(topic_id):
		body["topic_id"] = topic_id
	return await _write(ApiRequest.make_post(PATH_DUELS, body), "createDuel", "duel:%s:%s:%s" % [opponent_id, subject_id,
			topic_id], PackedStringArray(["SOCIAL_SCOPE_VIOLATION", "DUEL_EXPIRED"]))


## Coroutine: `acceptDuel` → the unranked match opens.
func accept_duel(duel: DuelModel) -> ApiResult:
	var result: ApiResult = await _write(ApiRequest.make_post(PATH_DUEL_ACCEPT % duel.duel_id), "acceptDuel",
			"duel_accept:%s" % duel.duel_id, PackedStringArray(["DUEL_EXPIRED", "HTTP_410"]))
	if result.ok:
		var accepted: DuelModel = DuelModel.from_dict(result.body())
		if accepted != null and Uuid.is_valid(accepted.match_id):
			_assign(accepted.match_id, "battle_1v1")
	return result


func decline_duel(duel: DuelModel) -> ApiResult:
	return await _write(ApiRequest.make_post(PATH_DUEL_DECLINE % duel.duel_id), "declineDuel", "",
			PackedStringArray(["DUEL_EXPIRED", "HTTP_410"]))


# ---------- realtime ----------

## Realtime dispatcher handler for `matchmaking.*` (GAME_NETWORKING §4.2).
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	match type:
		MSG_TICKET:
			_apply_ticket(MatchmakingTicketModel.from_dict(payload))
		MSG_PROPOSAL:
			var p: MatchProposalModel = MatchProposalModel.from_dict(payload)
			if p != null:
				proposal = p
				if ticket != null:
					ticket.status = "proposed"
				proposal_changed.emit(proposal)
		MSG_PROPOSAL_UPDATED:
			if proposal != null and DataRead.str_of(payload, "proposal_id") == proposal.proposal_id:
				proposal.accepted_count = maxi(0, DataRead.int_of(payload, "accepted_count"))
				proposal.total = maxi(proposal.total, DataRead.int_of(payload, "total"))
				proposal_changed.emit(proposal)
		MSG_PROPOSAL_CANCELLED:
			if proposal != null and DataRead.str_of(payload, "proposal_id") == proposal.proposal_id:
				var requeued: bool = DataRead.bool_of(payload, "requeued")
				proposal = null
				proposal_closed.emit(requeued)
				if requeued and ticket != null:
					ticket.status = "queued"
					ticket.requeued = true
					ticket_changed.emit(ticket)
				elif not requeued:
					_end_queue("cancelled")
		MSG_ASSIGNED:
			var match_id: String = DataRead.str_of(payload, "match_id")
			if Uuid.is_valid(match_id):
				proposal = null
				_end_queue("matched")
				_assign(match_id, DataRead.str_of(payload, "mode"))


## Seconds left of the ready check (server clock).
func proposal_seconds_left() -> int:
	return proposal.seconds_left(now_ms()) if proposal != null else 0


func now_ms() -> int:
	if realtime != null and realtime.server_offset_ms != 0:
		return realtime.server_now_ms()
	if api != null and api.clock != null:
		return int(api.clock.unix_time() * 1000.0)
	return int(Time.get_unix_time_from_system() * 1000.0)


func in_queue() -> bool:
	return ticket != null and ticket.is_active()


func is_busy() -> bool:
	return _busy > 0


## Opens a match the student entered another way (grade challenge `joinChallenge`, a tournament proposal).
func enter_match(match_id: String, mode_code: String) -> void:
	_assign(match_id, mode_code)


## Logout / takeover / new session: nothing of the previous student is kept.
func clear() -> void:
	modes.clear()
	modes_loaded = false
	subject_names.clear()
	topics.clear()
	ticket = null
	proposal = null
	assigned_match_id = ""
	assigned_mode = ""
	last_search = {}
	_intent_keys.clear()


# ---------- internals ----------

func _apply_ticket(next: MatchmakingTicketModel) -> void:
	if next == null:
		return
	if ticket != null and next.ticket_id != ticket.ticket_id and ticket.is_active() and not next.is_active():
		return
	if next.is_active():
		_set_ticket(next)
		return
	if next.status == "matched":
		ticket = null
		ticket_changed.emit(null)
		return
	var reason: String = "timeout" if next.timed_out() else "cancelled"
	_end_queue(reason)


func _set_ticket(next: MatchmakingTicketModel) -> void:
	if next == null:
		return
	ticket = next
	ticket_changed.emit(ticket)


func _end_queue(reason: String) -> void:
	var had: bool = ticket != null
	ticket = null
	if had:
		ticket_changed.emit(null)
		queue_ended.emit(reason)


func _assign(match_id: String, mode_code: String) -> void:
	if not Uuid.is_valid(match_id):
		return
	assigned_match_id = match_id
	assigned_mode = mode_code
	match_assigned.emit(match_id, mode_code)


## Coroutine: one REST write with the intent key rules of PartyService (R10).
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
		_notice(result.code)
	return result


func _notice(code: String) -> void:
	notice_requested.emit(CompetitionText.error_key(code), {})


func _set_busy(delta: int) -> void:
	var before: bool = _busy > 0
	_busy = maxi(0, _busy + delta)
	if before != (_busy > 0):
		busy_changed.emit(_busy > 0)


func _on_connection_state_changed(_state: int) -> void:
	var now: bool = realtime != null and realtime.is_reconnecting()
	if _was_reconnecting and not now and realtime.is_online() and ticket != null:
		refresh_ticket()
	_was_reconnecting = now
