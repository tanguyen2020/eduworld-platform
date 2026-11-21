class_name WorldAreaModel
extends RefCounted
## `WorldArea` of `listWorldAreas` (specs/openapi, F071): a stable `area_id`, its region, bilingual name `{vi, en}`, scene,
## whether the server lets this student in (`accessible`) and, when it does not, the reason and the requirements. The
## client never decides an unlock (ADR-0016): it only shows what the server returned. Names are data, never strings of
## the build (R2).

var area_id: String = ""
var region_code: String = ""
var name: Dictionary = {}
var region_name: Dictionary = {}
var area_class: String = ""
var subject_code: String = ""
var scene_ref: String = ""
var bundle_id: String = ""
var accessible: bool = false
var lock_reason: String = ""
var version: int = 0
var map_x: float = 0.5
var map_y: float = 0.5
var icon_key: String = ""
## `UnlockRequirement` dictionaries: kind, ref, label {vi, en}, met, current, target.
var requirements: Array[Dictionary] = []
## portal_id → target_area_id of the wind gates of the area.
var portals: Dictionary = {}


## Parses one `WorldArea`; null when it has no `area_id` or `scene_ref`.
static func from_dict(data: Variant) -> WorldAreaModel:
	if not data is Dictionary:
		return null
	var d: Dictionary = data as Dictionary
	if not d.get("area_id") is String or not d.get("scene_ref") is String:
		return null
	var a: WorldAreaModel = WorldAreaModel.new()
	a.area_id = String(d["area_id"])
	a.scene_ref = String(d["scene_ref"])
	a.region_code = _str(d.get("region_code"))
	a.name = d["name"] as Dictionary if d.get("name") is Dictionary else {}
	a.region_name = d["region_name"] as Dictionary if d.get("region_name") is Dictionary else {}
	a.area_class = _str(d.get("area_class"))
	a.subject_code = _str(d.get("subject_code"))
	a.bundle_id = _str(d.get("bundle_id"))
	a.accessible = d.get("accessible") == true
	a.lock_reason = _str(d.get("lock_reason"))
	a.version = int(d["version"]) if d.get("version") is float or d.get("version") is int else 0
	if d.get("map") is Dictionary:
		var m: Dictionary = d["map"] as Dictionary
		a.map_x = clampf(float(m.get("x", 0.5)), 0.0, 1.0)
		a.map_y = clampf(float(m.get("y", 0.5)), 0.0, 1.0)
		a.icon_key = _str(m.get("icon_key"))
	a.requirements = requirements_from(d.get("unlock_requirements"))
	if d.get("portals") is Array:
		for p: Variant in d["portals"] as Array:
			if p is Dictionary and (p as Dictionary).get("portal_id") is String:
				a.portals[String((p as Dictionary)["portal_id"])] = _str((p as Dictionary).get("target_area_id"))
	return a


## `UnlockRequirement` list (from an area or from `AREA_LOCKED.missing_requirements`).
static func requirements_from(data: Variant) -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	if data is Array:
		for r: Variant in data as Array:
			if r is Dictionary and (r as Dictionary).get("kind") is String:
				out.append(r as Dictionary)
	return out


## Display name in `locale` (falls back to vi, never to the raw id).
func display_name(locale: String = "") -> String:
	return L10n.pick(name, locale)


static func _str(value: Variant) -> String:
	return String(value) if value is String else ""
