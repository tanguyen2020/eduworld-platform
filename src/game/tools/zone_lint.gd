extends SceneTree
## CLI: `godot --headless --path src/game -s res://tools/zone_lint.gd` (F071-AC-13, T023). Exit 1 on violations.
## The known areas are the `area_id` of the world fixture (tests/fixtures/world/areas.json, from the backend seed).

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const ZoneRules := preload("res://tools/lib/zone_rules.gd")
const AREAS_FIXTURE: String = "res://tests/fixtures/world/areas.json"


func _initialize() -> void:
	var known: PackedStringArray = known_areas()
	var violations: PackedStringArray = []
	var zones: PackedStringArray = ZoneRules.zone_paths()
	for path: String in zones:
		violations.append_array(ZoneRules.check(path, known))
	quit(ToolFiles.report("zone_lint", violations, zones.size()))


static func known_areas() -> PackedStringArray:
	var out: PackedStringArray = []
	var data: Variant = JSON.parse_string(FileAccess.get_file_as_string(AREAS_FIXTURE))
	if data is Dictionary:
		for a: Variant in (data as Dictionary).get("data", []) as Array:
			if a is Dictionary:
				out.append(String((a as Dictionary).get("area_id", "")))
	return out
