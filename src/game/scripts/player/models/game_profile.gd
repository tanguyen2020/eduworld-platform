class_name GameProfileModel
extends RefCounted
## `GameProfile` of the signed-in student (specs/openapi `GameProfile`, F070): nickname (null until chosen from the
## server's suggestions), level/XP (server-computed, read only), `setup_required` (no avatar or no nickname yet →
## the client opens S02 Character Creation before the Main Hub, F070-AC-01) and the avatar. No real name, username,
## school or class (R16, ADR-0054). The nickname is never logged together with the student id (R14).

var student_id: String = ""
var nickname: String = ""
var level: int = 1
var xp_total: int = 0
var setup_required: bool = true
var avatar_config: AvatarConfigModel = null
var avatar_config_hash: String = ""


## Parses a `GameProfile`; null when the body is not one (no `student_id` or no `setup_required`).
static func from_dict(data: Variant) -> GameProfileModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	if not d.get("student_id") is String or not d.get("setup_required") is bool:
		return null
	var p: GameProfileModel = GameProfileModel.new()
	p.student_id = String(d["student_id"])
	p.nickname = String(d["nickname"]) if d.get("nickname") is String else ""
	p.level = _int(d.get("level"), 1)
	p.xp_total = _int(d.get("xp_total"), 0)
	p.setup_required = bool(d["setup_required"])
	p.avatar_config = AvatarConfigModel.from_dict(d.get("avatar_config"))
	p.avatar_config_hash = String(d["avatar_config_hash"]) if d.get("avatar_config_hash") is String else ""
	return p


## Fields shown by the HUD (mirrored into `AppState.player.game_profile`).
func to_display_dict() -> Dictionary:
	return {"student_id": student_id, "nickname": nickname if not nickname.is_empty() else null, "level": level,
			"xp_total": xp_total}


static func _int(value: Variant, fallback: int) -> int:
	return int(value) if value is int or value is float else fallback
