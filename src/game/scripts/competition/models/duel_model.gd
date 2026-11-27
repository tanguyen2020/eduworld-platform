class_name DuelModel
extends RefCounted
## A friend duel (`Duel`; F109-AC-27, MATCHMAKING §8): unranked, expires at the server `expires_at` (60 s).

var duel_id: String = ""
var challenger_id: String = ""
var opponent_id: String = ""
var subject_id: String = ""
var status: String = ""
var match_id: String = ""
var expires_ms: int = 0
## Nickname of the other student, filled by the screen from the friends list (never sent by the server here).
var other_nickname: String = ""


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
	m.match_id = DataRead.str_of(d, "match_id")
	var at: Variant = d.get("expires_at")
	m.expires_ms = ExamCountdown.iso_to_ms(at as String) if at is String else WsEnvelope.as_int(at, 0)
	return m


func seconds_left(now_ms: int) -> int:
	return maxi(0, ceili(float(expires_ms - now_ms) / 1000.0))


func is_pending() -> bool:
	return status == "pending"
