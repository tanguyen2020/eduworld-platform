class_name PublicPlayerModel
extends RefCounted
## Another player as the server may show them (AsyncAPI `PublicPlayer`; F092-AC-15, AGENTS R16): user id, nickname
## (≤ 32), avatar config, level. There is deliberately no field for a real name, school or class — unknown keys of the
## message are never copied, so nothing else can reach the UI.

const NICKNAME_MAX_LENGTH: int = 32

var user_id: String = ""
var nickname: String = ""
var avatar_config: Dictionary = {}
var level: int = 1


## Parses a `PublicPlayer`; null when `user_id` or `nickname` is missing or invalid.
static func from_dict(data: Variant) -> PublicPlayerModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	if not d.get("user_id") is String or not Uuid.is_valid(String(d["user_id"])):
		return null
	if not d.get("nickname") is String or String(d["nickname"]).length() > NICKNAME_MAX_LENGTH:
		return null
	var p: PublicPlayerModel = PublicPlayerModel.new()
	p.user_id = String(d["user_id"])
	p.nickname = String(d["nickname"])
	p.avatar_config = (d["avatar_config"] as Dictionary).duplicate(true) if d.get("avatar_config") is Dictionary else {}
	p.level = maxi(1, WsEnvelope.as_int(d.get("level"), 1))
	return p


## AvatarConfig for AvatarBuilder; null when the player sent none (the builder keeps its default look).
func avatar() -> AvatarConfigModel:
	return AvatarConfigModel.from_dict(avatar_config) if not avatar_config.is_empty() else null


## `hub.player.appearance_changed`: slot → item_id summary merged into the avatar config (no personal data).
func apply_appearance(appearance: Dictionary) -> void:
	for slot: Variant in appearance.keys():
		var value: Variant = appearance[slot]
		if not value is String or String(value).length() > 64:
			continue
		match String(slot):
			"body_type", "skin_tone":
				avatar_config[String(slot)] = String(value)
			"hair":
				var hair: Dictionary = avatar_config.get("hair", {}) as Dictionary if avatar_config.get("hair") is Dictionary else {}
				hair["item_id"] = String(value)
				avatar_config["hair"] = hair
			"hair_color":
				var hair_c: Dictionary = avatar_config.get("hair", {}) as Dictionary if avatar_config.get("hair") is Dictionary else {}
				hair_c["color"] = String(value)
				avatar_config["hair"] = hair_c
			"outfit":
				avatar_config["outfit"] = {"item_id": String(value)}
			"head", "face", "back", "hand":
				_set_accessory(String(slot), String(value))


func _set_accessory(slot: String, item_id: String) -> void:
	var list: Array = (avatar_config.get("accessories", []) as Array).duplicate() if avatar_config.get("accessories") is Array else []
	var out: Array = []
	for entry: Variant in list:
		if entry is Dictionary and String((entry as Dictionary).get("slot", "")) != slot:
			out.append(entry)
	if not item_id.is_empty():
		out.append({"slot": slot, "item_id": item_id})
	avatar_config["accessories"] = out
