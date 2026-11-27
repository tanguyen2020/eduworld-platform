class_name BattleClient
extends RefCounted
## The match on screen, server-authoritative (F109 US2/US3; GAME_NETWORKING §6–§7, ADR-0016, AGENTS R5/R6). Holds a
## BattleView rebuilt only from `battle.*` messages: `battle.state.snapshot` replaces everything (after a reconnect
## nothing local is reused, F109-AC-11), the other messages update one part. The student's answer is sent once
## (`battle.answer.submit`: match, question instance, raw response, `client_elapsed_ms` for reference only), the input
## locks at once and stays locked on `ANSWER_LATE` / `ANSWER_ALREADY_SUBMITTED` / `MATCH_NOT_ACTIVE` (never re-sent,
## F109-AC-06/07). Right / wrong / points / scores exist only after the server's `battle.answer.evaluated` /
## `battle.question.revealed`; no score is ever sent (R5). Updates are event driven, no polling (F109-AC-12).
## A message with an older `state_version` is dropped; a version gap asks for a snapshot (spec edge cases).

signal view_changed(view: BattleView)
signal question_changed(view: BattleView)
signal answer_locked()
signal answer_evaluated(evaluation: AnswerEvaluation)
signal question_revealed(view: BattleView)
signal scores_changed(view: BattleView)
signal player_status_changed(user_id: String, connected: bool, grace_until: int)
signal paused_changed(paused: bool, resume_deadline_at: int)
signal match_ended(summary: MatchSummary)
signal notice_requested(key: String, args: Dictionary)

const PREFIX: String = "battle"
const CMD_SUBMIT: String = "battle.answer.submit"
const CMD_STATE: String = "battle.state.request"
## Refusals that keep the answer locked (F109-AC-07): the server already has an answer / the time is over.
const KEEP_LOCKED: PackedStringArray = ["ANSWER_LATE", "ANSWER_ALREADY_SUBMITTED", "MATCH_NOT_ACTIVE"]

var realtime: GameRealtimeClient = null
var app_state: Node = null
var match_id: String = ""
var mode: String = ""
var view: BattleView = null
var summary: MatchSummary = null
var self_id: String = ""
## The student's answer is on its way / was accepted: the input stays locked until the next question.
var input_locked: bool = false
## Commands sent (tests count them; one answer per question).
var submits: int = 0
var _paused_state: String = ""
var _was_reconnecting: bool = false


func _init(realtime_client: GameRealtimeClient = null, app_state_node: Node = null) -> void:
	realtime = realtime_client
	app_state = app_state_node
	if realtime != null:
		realtime.register_handler(PREFIX, handle_message)
		realtime.connection_state_changed.connect(_on_connection_state_changed)


## Enters `match_id`: forgets the previous match and asks the server for the full state (also after an app restart
## inside the grace — F109 edge case).
func start(next_match_id: String, next_mode: String = "") -> void:
	match_id = next_match_id
	mode = next_mode
	view = null
	summary = null
	input_locked = false
	self_id = CompetitionPlayer.self_id(app_state)
	request_state()


## Coroutine: `battle.state.request` (snapshot comes back as `battle.state.snapshot`).
func request_state() -> bool:
	if realtime == null or not Uuid.is_valid(match_id):
		return false
	var outcome: CommandResult = await realtime.send_command(CMD_STATE, {"match_id": match_id})
	if not outcome.ok and outcome.code == "NOT_FOUND":
		notice_requested.emit(CompetitionText.error_key("NOT_FOUND"), {})
	return outcome.ok


## Coroutine: the student's answer (tap or key 1–4). Exactly one submit per question; the screen shows "Đã ghi nhận"
## from `answer_locked`. Returns true when the server took it.
func submit_answer(response: Dictionary) -> bool:
	if view == null or view.state != BattleView.STATE_QUESTION or input_locked or view.my_answer_locked or response.is_empty():
		return false
	input_locked = true
	answer_locked.emit()
	var payload: Dictionary = {"match_id": match_id, "question_instance_id": view.question_instance_id, "response": response,
		"client_elapsed_ms": maxi(0, now_ms() - view.shown_at) if view.shown_at > 0 else 0}
	submits += 1
	var instance: String = view.question_instance_id
	var outcome: CommandResult = await realtime.send_command(CMD_SUBMIT, payload) if realtime != null \
			else CommandResult.failure(CommandResult.CODE_OFFLINE)
	if view == null or view.question_instance_id != instance:
		return outcome.ok
	if outcome.ok:
		view.my_answer_locked = true
		return true
	if outcome.code in KEEP_LOCKED or outcome.code == CommandResult.CODE_TIMEOUT:
		view.my_answer_locked = outcome.code != CommandResult.CODE_TIMEOUT
		notice_requested.emit(CompetitionText.error_key(outcome.code), {})
		if outcome.code == CommandResult.CODE_TIMEOUT:
			request_state()
		return false
	# Not sent at all (offline, invalid frame): the student may answer again once the connection is back.
	input_locked = false
	notice_requested.emit(CompetitionText.error_key(outcome.code), {})
	view_changed.emit(view)
	return false


## Realtime dispatcher handler for `battle.*`.
func handle_message(type: String, payload: Dictionary, _envelope: Variant = null) -> void:
	var id: String = DataRead.str_of(payload, "match_id")
	if id != match_id or match_id.is_empty():
		return
	if WsEnvelope.is_int(payload.get("state_version")) and type != "battle.state.snapshot" and view != null:
		var version: int = DataRead.int_of(payload, "state_version")
		if version <= view.state_version:
			return
		if version > view.state_version + 1:
			request_state()
			return
		view.state_version = version
	match type:
		"battle.state.snapshot":
			_apply_snapshot(payload)
		"battle.countdown.started":
			_ensure_view()
			view.state = BattleView.STATE_COUNTDOWN
			view.countdown_at = DataRead.int_of(payload, "starts_at")
			view.question_count = maxi(view.question_count, DataRead.int_of(payload, "question_count"))
			view_changed.emit(view)
		"battle.question.shown":
			_ensure_view()
			var q: Dictionary = DataRead.dict_of(payload, "question")
			if q.is_empty():
				return
			view.set_question(q, maxi(0, DataRead.int_of(payload, "question_index")))
			view.state = BattleView.STATE_QUESTION
			view.shown_at = DataRead.int_of(payload, "shown_at", now_ms())
			view.deadline_at = DataRead.int_of(payload, "deadline_at")
			view.my_answer_locked = false
			if payload.has("primary_user_ids"):
				view.primary_user_ids = DataRead.strings_of(payload, "primary_user_ids", 8)
			input_locked = false
			question_changed.emit(view)
		"battle.answer.recorded":
			if view == null:
				return
			var who: String = DataRead.str_of(payload, "user_id")
			if who == self_id:
				view.my_answer_locked = true
				input_locked = true
				answer_locked.emit()
			else:
				var p: BattlePlayer = view.player(who)
				if p != null:
					p.answered = true
				scores_changed.emit(view)
		"battle.answer.evaluated":
			if view == null:
				return
			var e: AnswerEvaluation = AnswerEvaluation.from_dict(payload)
			if e != null:
				view.evaluation = e
				input_locked = true
				answer_evaluated.emit(e)
		"battle.question.revealed":
			if view == null:
				return
			view.state = BattleView.STATE_REVEAL
			view.correct_rate = DataRead.num_of(payload, "correct_rate", -1.0)
			_apply_scores(DataRead.arr_of(payload, "scoreboard"))
			input_locked = true
			question_revealed.emit(view)
		"battle.round.ended":
			if view == null:
				return
			view.state = BattleView.STATE_ROUND_END
			_apply_scores(DataRead.arr_of(payload, "standings"))
		"battle.participant.status_changed":
			if view == null:
				return
			var who: String = DataRead.str_of(payload, "user_id")
			var p: BattlePlayer = view.player(who)
			if p != null:
				p.connected = DataRead.bool_of(payload, "connected", true)
				p.grace_until = DataRead.int_of(payload, "grace_until") if not p.connected else 0
				player_status_changed.emit(who, p.connected, p.grace_until)
		"battle.match.paused":
			_ensure_view()
			if view.state != BattleView.STATE_PAUSED:
				_paused_state = view.state
			view.state = BattleView.STATE_PAUSED
			view.resume_deadline_at = DataRead.int_of(payload, "resume_deadline_at")
			paused_changed.emit(true, view.resume_deadline_at)
		"battle.match.resumed":
			if view == null:
				return
			view.state = _paused_state if not _paused_state.is_empty() else BattleView.STATE_QUESTION
			_paused_state = ""
			paused_changed.emit(false, 0)
			if DataRead.bool_of(payload, "replay_current_question") and view.question != null:
				# Failover: the unfinished question is played again — unlocked and shown again (spec edge case).
				view.state = BattleView.STATE_QUESTION
				view.my_answer_locked = false
				view.evaluation = null
				input_locked = false
				question_changed.emit(view)
		"battle.match.ended":
			var s: MatchSummary = MatchSummary.from_dict(payload)
			if s == null:
				return
			summary = s
			_ensure_view()
			view.state = BattleView.STATE_CANCELLED if s.is_void() else BattleView.STATE_ENDED
			for r: Dictionary in s.results:
				var p: BattlePlayer = view.player(String(r["user_id"]))
				if p != null and p.score_visible():
					p.score = int(r["score"])
			input_locked = true
			match_ended.emit(s)


func now_ms() -> int:
	return realtime.server_now_ms() if realtime != null else int(Time.get_unix_time_from_system() * 1000.0)


## Players other than the student (chat members, result safety row): nickname / avatar only.
func others() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	if view != null:
		for p: BattlePlayer in view.participants:
			if p.user_id != self_id and not p.bot:
				out.append(p.to_card())
	return out


func my_team() -> int:
	var me: BattlePlayer = view.player(self_id) if view != null else null
	return me.team if me != null else 0


func is_primary() -> bool:
	return view != null and self_id in view.primary_user_ids


func leave() -> void:
	match_id = ""
	mode = ""
	view = null
	summary = null
	input_locked = false


func clear() -> void:
	leave()
	submits = 0


func _apply_snapshot(payload: Dictionary) -> void:
	var next: BattleView = BattleView.from_snapshot(payload)
	if next == null:
		return
	if view != null and next.state_version < view.state_version:
		return
	if view != null and next.mode.is_empty():
		next.mode = view.mode
	if next.mode.is_empty():
		next.mode = mode
	if view != null and view.question_instance_id == next.question_instance_id:
		next.shown_at = view.shown_at
	if next.shown_at == 0 and next.time_limit_ms > 0 and next.deadline_at > 0:
		next.shown_at = next.deadline_at - next.time_limit_ms
	view = next
	input_locked = view.my_answer_locked or view.state != BattleView.STATE_QUESTION
	view_changed.emit(view)
	if view.state == BattleView.STATE_QUESTION:
		question_changed.emit(view)


func _apply_scores(rows: Array) -> void:
	for raw: Variant in rows:
		if raw is Dictionary:
			var p: BattlePlayer = view.player(DataRead.str_of(raw as Dictionary, "user_id"))
			var score: Variant = (raw as Dictionary).get("score")
			if p != null and (score is int or score is float):
				p.score = int(score)
	scores_changed.emit(view)


func _ensure_view() -> void:
	if view == null:
		view = BattleView.new()
		view.match_id = match_id
		view.mode = mode


func _on_connection_state_changed(_state: int) -> void:
	var now: bool = realtime != null and realtime.is_reconnecting()
	if _was_reconnecting and not now and realtime.is_online() and Uuid.is_valid(match_id) \
			and (view == null or not view.is_over()):
		# F109-AC-11: back online — the whole screen is rebuilt from the snapshot.
		request_state()
	_was_reconnecting = now
