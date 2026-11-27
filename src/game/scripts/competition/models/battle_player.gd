class_name BattlePlayer
extends RefCounted
## One seat of a match as the server shows it (`battle.state.snapshot.participants[]`; F109-AC-05, AC-10, AC-11):
## nickname + avatar only (AGENTS R16), team, score (absent = hidden by the server, e.g. other students in a Timed
## Quiz), connection and the reconnect grace the server granted. The practice robot carries `bot` (always labelled).

const SCORE_HIDDEN: int = -1

var user_id: String = ""
var nickname: String = ""
var avatar_config: Dictionary = {}
var team: int = 0
var score: int = SCORE_HIDDEN
var connected: bool = true
var grace_until: int = 0
var bot: bool = false
## Answered the current question (`battle.answer.recorded`, content never shown).
var answered: bool = false


static func from_dict(data: Variant) -> BattlePlayer:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var player: PublicPlayerModel = PublicPlayerModel.from_dict(d.get("player"))
	if player == null:
		return null
	var p: BattlePlayer = BattlePlayer.new()
	p.user_id = player.user_id
	p.nickname = player.nickname
	p.avatar_config = player.avatar_config
	p.team = DataRead.int_of(d, "team")
	p.score = DataRead.int_of(d, "score", SCORE_HIDDEN) if (d.get("score") is int or d.get("score") is float) else SCORE_HIDDEN
	p.connected = DataRead.bool_of(d, "connected", true)
	p.grace_until = DataRead.int_of(d, "grace_until")
	p.bot = DataRead.bool_of(d, "bot")
	return p


func score_visible() -> bool:
	return score != SCORE_HIDDEN


## `{user_id, nickname, avatar_config}` for the chat dock and the result safety row (never the student).
func to_card() -> Dictionary:
	return {"user_id": user_id, "nickname": nickname, "avatar_config": avatar_config}
