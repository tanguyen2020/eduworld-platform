class_name ChallengeAttemptModel
extends RefCounted
## One solo challenge run (`GameChallengeAttempt`, AsyncAPI `challenge.attempt.updated`; CHALLENGE_BOSS_SYSTEM §2).
## Status, score, stamina, progress and deadline are the server's (ADR-0016): the client only shows them and only
## counts `activity_seq` locally to build the Idempotency-Key of each `nextChallengeActivity` call.

const STATUS_IN_PROGRESS: String = "in_progress"
const STATUS_SUCCEEDED: String = "succeeded"
const STATUS_FAILED: String = "failed"
const STATUS_ABANDONED: String = "abandoned"
const STATUSES: PackedStringArray = ["in_progress", "succeeded", "failed", "abandoned"]

var challenge_attempt_id: String = ""
var challenge_id: String = ""
var challenge_version: int = 0
var difficulty: String = ""
var status: String = ""
var score: float = 0.0
## -1 = not sent (no stamina rule for this challenge).
var lives_left: int = -1
var activities_completed: int = 0
var started_at: String = ""
var deadline_at: String = ""
var ended_at: String = ""
var rewards: Array[RewardModel] = []
## Local count of `nextChallengeActivity` calls of this run (key per turn, never sent as data).
var activity_seq: int = 0


static func from_dict(d: Variant) -> ChallengeAttemptModel:
	if not d is Dictionary:
		return null
	var m: Dictionary = d as Dictionary
	var a: ChallengeAttemptModel = ChallengeAttemptModel.new()
	a.challenge_attempt_id = DataRead.str_of(m, "challenge_attempt_id")
	a.status = DataRead.str_of(m, "status")
	if not Uuid.is_valid(a.challenge_attempt_id) or not a.status in STATUSES:
		return null
	a.challenge_id = DataRead.str_of(m, "challenge_id")
	a.challenge_version = DataRead.int_of(m, "challenge_version")
	a.difficulty = DataRead.str_of(m, "difficulty")
	a.started_at = DataRead.str_of(m, "started_at")
	a.apply_update(m)
	return a


## Applies a `challenge.attempt.updated` payload or a fresh `GameChallengeAttempt` (server values replace).
func apply_update(m: Dictionary) -> void:
	var next_status: String = DataRead.str_of(m, "status", status)
	if next_status in STATUSES:
		status = next_status
	score = maxf(0.0, DataRead.num_of(m, "score", score))
	if m.has("lives_left"):
		lives_left = DataRead.nullable_int(m, "lives_left")
	activities_completed = maxi(0, DataRead.int_of(m, "activities_completed", activities_completed))
	deadline_at = DataRead.str_of(m, "deadline_at", deadline_at)
	ended_at = DataRead.str_of(m, "ended_at", ended_at)
	if m.get("rewards") is Array:
		rewards = RewardModel.list_from(m["rewards"])


func is_finished() -> bool:
	return status != STATUS_IN_PROGRESS


func has_lives() -> bool:
	return lives_left >= 0
