class_name ChallengeDetailModel
extends ChallengeSummaryModel
## Details of one challenge (`GameChallengeDetail`): description, rules, time limit, stamina, cooldown and first-clear
## rewards, plus (contract §6 delta) one entry per difficulty with `available` and the server's lock reason. A locked
## difficulty is shown only when the server gave its reason — the client never infers grade rules (F069-AC-14,
## AC-UX-0601, Clarifications 2026-11-16).

## Translation keys of the delta lock reasons, listed literally for tools/extract_pot.gd.
const LOCK_KEYS: PackedStringArray = [
	"ui.challenge.locked.grade_not_allowed", "ui.challenge.locked.not_unlocked", "ui.challenge.locked.school_disabled",
]

var description: Dictionary = {}
var rules_summary: Dictionary = {}
var version: int = 0
var time_limit_seconds: int = 0
var lives: int = 0
var cooldown_until: String = ""
var first_clear_rewards: Array[RewardModel] = []
## `{difficulty, available, locked_reason_code, time_limit_seconds, lives, reward_multiplier}` (delta; may be empty).
var difficulties: Array[Dictionary] = []


static func detail_from(d: Variant) -> ChallengeDetailModel:
	var c: ChallengeDetailModel = ChallengeDetailModel.new()
	if not c.parse(d):
		return null
	var m: Dictionary = d as Dictionary
	c.description = DataRead.text_of(m, "description")
	c.rules_summary = DataRead.text_of(m, "rules_summary")
	c.version = DataRead.int_of(m, "version")
	c.time_limit_seconds = maxi(0, DataRead.int_of(m, "time_limit_seconds"))
	c.lives = maxi(0, DataRead.int_of(m, "lives"))
	c.cooldown_until = DataRead.str_of(m, "cooldown_until")
	c.first_clear_rewards = RewardModel.list_from(m.get("first_clear_rewards"))
	for raw: Variant in DataRead.arr_of(m, "difficulties"):
		if not raw is Dictionary:
			continue
		var e: Dictionary = raw as Dictionary
		var level: String = DataRead.str_of(e, "difficulty")
		if not level in DIFFICULTIES or c.difficulties.size() >= DIFFICULTIES.size():
			continue
		c.difficulties.append({"difficulty": level, "available": DataRead.bool_of(e, "available"),
			"locked_reason_code": DataRead.str_of(e, "locked_reason_code"),
			"time_limit_seconds": maxi(0, DataRead.int_of(e, "time_limit_seconds")),
			"lives": maxi(0, DataRead.int_of(e, "lives")), "reward_multiplier": DataRead.num_of(e, "reward_multiplier")})
	return c


## Difficulties to draw: `{difficulty, locked: bool, reason_key}` — the available ones, plus the locked ones the server
## explained. Without the delta, exactly `available_difficulties`.
func difficulty_options() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	if difficulties.is_empty():
		for level: String in available_difficulties:
			out.append({"difficulty": level, "locked": false, "reason_key": ""})
		return out
	for e: Dictionary in difficulties:
		var level: String = String(e["difficulty"])
		if bool(e["available"]) and level in available_difficulties:
			out.append({"difficulty": level, "locked": false, "reason_key": ""})
			continue
		var key: String = "ui.challenge.locked." + String(e["locked_reason_code"])
		if not bool(e["available"]) and key in LOCK_KEYS:
			out.append({"difficulty": level, "locked": true, "reason_key": key})
	return out


## Reward multiplier of `level` when the server sent it (g6_9 shows it, S06 §10); 0 when unknown.
func multiplier_of(level: String) -> float:
	for e: Dictionary in difficulties:
		if String(e["difficulty"]) == level:
			return float(e["reward_multiplier"])
	return 0.0


func description_text() -> String:
	return L10n.pick(description)


func rules_text() -> String:
	return L10n.pick(rules_summary)
