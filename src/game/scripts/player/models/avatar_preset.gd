class_name AvatarPresetModel
extends RefCounted
## One saved outfit (`AvatarPreset`, specs/openapi; CHARACTER_SYSTEM §5.1: at most 3 saved outfits): slot 1…3, an
## optional name and the full `AvatarConfig`.

var slot: int = 0
var name: String = ""
var avatar_config: AvatarConfigModel = null


static func list_from(data: Variant) -> Array[AvatarPresetModel]:
	var out: Array[AvatarPresetModel] = []
	if not data is Array:
		return out
	for entry: Variant in data as Array:
		if not entry is Dictionary:
			continue
		var d: Dictionary = entry as Dictionary
		var config: AvatarConfigModel = AvatarConfigModel.from_dict(d.get("avatar_config"))
		if config == null or not (d.get("slot") is int or d.get("slot") is float):
			continue
		var p: AvatarPresetModel = AvatarPresetModel.new()
		p.slot = int(d["slot"])
		p.name = String(d["name"]) if d.get("name") is String else ""
		p.avatar_config = config
		out.append(p)
	return out
