class_name DuelModel
extends RefCounted
## A friend duel (`Duel`; F109-AC-27, MATCHMAKING §8): unranked, expires at the server `expires_at` (60 s). Built from
## the REST `Duel` or from the realtime `competition.duel.received` (the duel a friend sent this student: the
## challenger's public card only — nickname, avatar, level; AGENTS R16).

var duel_id: String = ""
var challenger_id: String = ""
var opponent_id: String = ""
var subject_id: String = ""
var topic_id: String = ""
var status: String = ""
var match_id: String = ""
var expires_ms: int = 0
## Nickname of the other student: the challenger card of `competition.duel.received`, else filled by the screen from the
## friends list (the REST `Duel` carries ids only).
var other_nickname: String = ""
## Avatar config and level of the challenger card (`PublicPlayer`), empty / 0 when unknown.
var other_avatar: Dictionary = {}
var other_level: int = 0


static func from_dict(data: Variant) -> DuelModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var m: DuelModel = DuelModel.new()
	m.duel_id = DataRead.str_of(d, "duel_id")
	m.status = DataRead.str_of(d, "status")
	if not Uuid.is_valid(m.duel_id) or m.status.is_empty():
		return null
	m.challenger_id = DataRead.str_of(d, "challenger_id")
	m.opponent_id = DataRead.str_of(d, "opponent_id")
	m.subject_id = DataRead.str_of(d, "subject_id")
	m.topic_id = DataRead.str_of(d, "topic_id")
	m.match_id = DataRead.str_of(d, "match_id")
	var at: Variant = d.get("expires_at")
	m.expires_ms = ExamCountdown.iso_to_ms(at as String) if at is String else WsEnvelope.as_int(at, 0)
	return m


## `competition.duel.received` → a pending duel from `challenger`; null when a required field is missing or invalid.
static func from_received(data: Variant) -> DuelModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var challenger: PublicPlayerModel = PublicPlayerModel.from_dict(d.get("challenger"))
	var m: DuelModel = DuelModel.new()
	m.duel_id = DataRead.str_of(d, "duel_id")
	m.subject_id = DataRead.str_of(d, "subject_id")
	if challenger == null or not Uuid.is_valid(m.duel_id) or not Uuid.is_valid(m.subject_id) \
			or not WsEnvelope.is_int(d.get("expires_at")):
		return null
	m.status = "pending"
	m.challenger_id = challenger.user_id
	m.other_nickname = challenger.nickname
	m.other_avatar = challenger.avatar_config
	m.other_level = challenger.level if (d["challenger"] as Dictionary).has("level") else 0
	var topic: String = DataRead.str_of(d, "topic_id")
	m.topic_id = topic if Uuid.is_valid(topic) else ""
	m.expires_ms = WsEnvelope.as_int(d["expires_at"], 0)
	return m


func seconds_left(now_ms: int) -> int:
	return maxi(0, ceili(float(expires_ms - now_ms) / 1000.0))


func is_pending() -> bool:
	return status == "pending"
