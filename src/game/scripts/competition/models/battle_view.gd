class_name BattleView
extends RefCounted
## What the battle screen shows, rebuilt from the server only (F109 US2; GAME_NETWORKING §7; ADR-0016): the full
## `battle.state.snapshot` replaces everything (after a reconnect nothing local is reused, F109-AC-11) and each
## `battle.*` message updates one part. Scores, progress, deadlines and verdicts are never computed here.
## The server fields of the 2026-11-27 delta (spec Clarifications): `speed_bonus_enabled` (countdown / snapshot — the
## speed hint shows only when true, F109-AC-09), `primary_user_ids` (team battle — the "lượt chính" marker, AC-10) and
## `chat_channel_ids` {match, team?} (the always-on match chat, AC-13). An older server without them reads safely:
## no speed hint, no primary marker, and the chat falls back to the open channels of `listMyChannels`.

## Display states (data-model §2) mapped from `battle.state.snapshot.state`.
const STATE_LOBBY: String = "lobby"
const STATE_COUNTDOWN: String = "countdown"
const STATE_QUESTION: String = "question"
const STATE_REVEAL: String = "reveal"
const STATE_ROUND_END: String = "round_end"
const STATE_PAUSED: String = "paused"
const STATE_FINALIZING: String = "finalizing"
const STATE_ENDED: String = "ended"
const STATE_CANCELLED: String = "cancelled"
const SERVER_STATES: Dictionary = {
	"lobby": STATE_LOBBY, "ready_check": STATE_LOBBY, "countdown": STATE_COUNTDOWN, "question_shown": STATE_QUESTION,
	"reveal": STATE_REVEAL, "round_end": STATE_ROUND_END, "paused": STATE_PAUSED, "finalizing": STATE_FINALIZING,
	"ended": STATE_ENDED, "cancelled": STATE_CANCELLED,
}
## Keys of the server question that are not part of the F063 render payload.
const QUESTION_META: PackedStringArray = ["question_instance_id", "question_type", "time_limit_ms"]

var match_id: String = ""
var mode: String = ""
var state: String = STATE_LOBBY
var state_version: int = 0
var question_index: int = 0
var question_count: int = 0
var question: QuestionRenderModel = null
var question_instance_id: String = ""
var shown_at: int = 0
var time_limit_ms: int = 0
var deadline_at: int = 0
var countdown_at: int = 0
var my_answer_locked: bool = false
var participants: Array[BattlePlayer] = []
var speed_bonus_enabled: bool = false
var primary_user_ids: PackedStringArray = []
## `chat_channel_ids` of the server: `{match}` (+ `team` in a team battle, the student's own team) — valid ids only.
var chat_channel_ids: Dictionary = {}
var resume_deadline_at: int = 0
var evaluation: AnswerEvaluation = null
var correct_rate: float = -1.0


## A full snapshot; null when a required field is missing (the message is dropped, GAME_NETWORKING §9).
static func from_snapshot(data: Dictionary) -> BattleView:
	var v: BattleView = BattleView.new()
	v.match_id = DataRead.str_of(data, "match_id")
	var raw_state: String = DataRead.str_of(data, "state")
	if not Uuid.is_valid(v.match_id) or not SERVER_STATES.has(raw_state) or not WsEnvelope.is_int(data.get("state_version")) \
			or not data.get("participants") is Array:
		return null
	v.state = String(SERVER_STATES[raw_state])
	v.state_version = DataRead.int_of(data, "state_version")
	v.mode = DataRead.str_of(data, "mode")
	v.question_index = maxi(0, DataRead.int_of(data, "question_index"))
	v.question_count = maxi(0, DataRead.int_of(data, "question_count"))
	v.deadline_at = DataRead.int_of(data, "deadline_at")
	v.my_answer_locked = DataRead.bool_of(data, "my_answer_locked")
	v.speed_bonus_enabled = DataRead.bool_of(data, "speed_bonus_enabled")
	v.primary_user_ids = DataRead.strings_of(data, "primary_user_ids", 8)
	v.chat_channel_ids = chat_channels_of(data)
	for raw: Variant in data["participants"] as Array:
		var p: BattlePlayer = BattlePlayer.from_dict(raw)
		if p != null:
			v.participants.append(p)
	if data.get("current_question") is Dictionary and v.state == STATE_QUESTION:
		v.set_question(data["current_question"] as Dictionary, v.question_index)
	return v


## `chat_channel_ids` of a `matchmaking.match.assigned` / `battle.state.snapshot` payload (`MatchChatChannelIds`):
## `{match}` + `team` when sent; empty when absent or the match id is not a UUID (the client never builds an id).
static func chat_channels_of(data: Dictionary) -> Dictionary:
	var raw: Dictionary = DataRead.dict_of(data, "chat_channel_ids")
	var match_channel: String = DataRead.str_of(raw, "match")
	if not Uuid.is_valid(match_channel):
		return {}
	var out: Dictionary = {"match": match_channel}
	var team_channel: String = DataRead.str_of(raw, "team")
	if Uuid.is_valid(team_channel):
		out["team"] = team_channel
	return out


## The server question (`QuestionRender` of the AsyncAPI) as the F063 model: the render fields as `render_payload`,
## options keyed `id` (the standard key since 2026-11-27; the deprecated `option_id` alias is still accepted when
## it comes alone). Answer keys a server bug might add are stripped by QuestionRenderModel (R6).
static func question_from(q: Dictionary, index: int) -> QuestionRenderModel:
	var payload: Dictionary = {}
	for k: Variant in q.keys():
		if not String(k) in QUESTION_META and String(k) != "media":
			payload[k] = q[k]
	var options: Array = []
	for o: Variant in DataRead.arr_of(q, "options"):
		if o is Dictionary:
			var opt: Dictionary = (o as Dictionary).duplicate(true)
			if not opt.get("id") is String and opt.get("option_id") is String:
				opt["id"] = opt["option_id"]
			options.append(opt)
	if not options.is_empty():
		payload["options"] = options
	return QuestionRenderModel.from_dict({"position": index + 1, "question_version_id": DataRead.str_of(q,
			"question_instance_id"), "type": DataRead.str_of(q, "question_type"), "render_payload": payload,
			"media": DataRead.arr_of(q, "media")})


func set_question(q: Dictionary, index: int) -> void:
	question_instance_id = DataRead.str_of(q, "question_instance_id")
	time_limit_ms = maxi(0, DataRead.int_of(q, "time_limit_ms"))
	question = question_from(q, index)
	question_index = index
	evaluation = null
	correct_rate = -1.0
	for p: BattlePlayer in participants:
		p.answered = false


func player(user_id: String) -> BattlePlayer:
	for p: BattlePlayer in participants:
		if p.user_id == user_id:
			return p
	return null


func is_team_battle() -> bool:
	return mode == "team_battle"


## Team numbers in order (1v1: one seat per team).
func teams() -> Array[int]:
	var out: Array[int] = []
	for p: BattlePlayer in participants:
		if not p.team in out:
			out.append(p.team)
	out.sort()
	return out


## Sum of the server scores of `team` (hidden scores are not counted); -1 when no score of the team is visible.
func team_score(team: int) -> int:
	var total: int = 0
	var any: bool = false
	for p: BattlePlayer in participants:
		if p.team == team and p.score_visible():
			total += p.score
			any = true
	return total if any else BattlePlayer.SCORE_HIDDEN


func members(team: int) -> Array[BattlePlayer]:
	var out: Array[BattlePlayer] = []
	for p: BattlePlayer in participants:
		if p.team == team:
			out.append(p)
	return out


## Milliseconds left of the question at server time `now_ms`.
func ms_left(now_ms: int) -> int:
	return maxi(0, deadline_at - now_ms)


func total_ms() -> int:
	if time_limit_ms > 0:
		return time_limit_ms
	return maxi(1, deadline_at - shown_at) if shown_at > 0 and deadline_at > shown_at else 1


func is_over() -> bool:
	return state == STATE_ENDED or state == STATE_CANCELLED
