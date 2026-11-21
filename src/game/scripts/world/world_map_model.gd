class_name WorldMapModel
extends RefCounted
## Data of the World Map S04 (STUDENT_GAME_UX_CORE S04, F071-AC-26/27) built from what the server returned
## (`listWorldAreas` + `getWorldExploration`): one entry per visible area with its open/locked state, the conditions in
## words, the exploration percentage (`exploration_progress`) and whether fast travel is offered. Grade band `g1_3`
## gets the picture board: open areas only, no percentage, one tap to travel. There is never an option to open an area
## with coins (FR-GAM-009).

const BAND_PICTURE: String = "g1_3"

var picture_mode: bool = false
## Entries: area_id, name {vi,en}, region_code, accessible, pct (float or -1 when hidden), requirements (Array of
## {kind, label, current, target}), can_travel, map_x, map_y, icon_key, current.
var entries: Array[Dictionary] = []


static func build(areas: Array[WorldAreaModel], exploration: Dictionary, grade_band: String, current_area: String) -> WorldMapModel:
	var m: WorldMapModel = WorldMapModel.new()
	m.picture_mode = grade_band == BAND_PICTURE
	var explored: Dictionary = {}
	for x: Variant in exploration.get("areas", []) as Array:
		if x is Dictionary:
			explored[String((x as Dictionary).get("area_id", ""))] = x
	for a: WorldAreaModel in areas:
		if m.picture_mode and not a.accessible:
			continue
		var x: Dictionary = explored.get(a.area_id, {}) as Dictionary
		var reqs: Array[Dictionary] = []
		if not a.accessible:
			var source: Array = x.get("unlock_requirements", []) as Array if x.has("unlock_requirements") else a.requirements
			for r: Variant in source:
				if r is Dictionary and not bool((r as Dictionary).get("met", false)):
					reqs.append(r as Dictionary)
		m.entries.append({"area_id": a.area_id, "name": a.name, "region_code": a.region_code, "accessible": a.accessible,
			"lock_reason": a.lock_reason, "pct": -1.0 if m.picture_mode else float(x.get("completion_pct", 0.0)),
			"requirements": reqs, "can_travel": a.accessible and a.area_id != current_area, "map_x": a.map_x, "map_y": a.map_y,
			"icon_key": a.icon_key, "current": a.area_id == current_area})
	return m


## Condition in words: the label of the data in the player's language, else the generic key of its kind with the
## progress (`ui.worldmap.req.<kind>`).
static func requirement_text(r: Dictionary, locale: String = "") -> String:
	if r.get("label") is Dictionary and not (r["label"] as Dictionary).is_empty():
		return L10n.pick(r["label"] as Dictionary, locale)
	return TranslationServer.translate("ui.worldmap.req." + String(r.get("kind", "quest"))).format(
			{"current": int(r.get("current", 0)), "target": int(r.get("target", 1))})
