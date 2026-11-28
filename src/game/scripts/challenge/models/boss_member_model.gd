class_name BossMemberModel
extends RefCounted
## One member of a boss encounter as the server shows it (OpenAPI `BossEncounter.members[]`, AsyncAPI
## `boss.encounter.updated.members[]`; F110 data-model §1, AGENTS R16): id, nickname, ready, connected and whether the
## member reached the contribution of the phase (`contribution_met` — a tick, never a count, CHALLENGE_BOSS_SYSTEM §3.5).
## The REST form says `present` where the realtime form says `connected`. Nothing else of the member is copied.

const NICKNAME_MAX_LENGTH: int = 32

var student_id: String = ""
var nickname: String = ""
var ready: bool = false
var connected: bool = true
var contribution_met: bool = false


static func from_dict(data: Variant) -> BossMemberModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	var id: String = DataRead.str_of(d, "student_id")
	var nick: String = DataRead.str_of(d, "nickname")
	if not Uuid.is_valid(id) or nick.is_empty() or nick.length() > NICKNAME_MAX_LENGTH:
		return null
	var m: BossMemberModel = BossMemberModel.new()
	m.student_id = id
	m.nickname = nick
	m.ready = DataRead.bool_of(d, "ready")
	m.connected = DataRead.bool_of(d, "connected", DataRead.bool_of(d, "present", true))
	m.contribution_met = DataRead.bool_of(d, "contribution_met")
	return m


static func list_from(data: Variant) -> Array[BossMemberModel]:
	var out: Array[BossMemberModel] = []
	if data is Array:
		for raw: Variant in data as Array:
			var m: BossMemberModel = from_dict(raw)
			if m != null and out.size() < 8:
				out.append(m)
	return out
