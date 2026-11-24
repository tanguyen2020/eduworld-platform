extends SceneTree
## CLI: `godot --headless --path src/game -s res://tools/lint_art.gd` (F067-AC-03, F067-AC-04; ADR-0102, R32).
## Fails when a graphic asset has no row in res://assets/LICENSES.md or a licence outside the allow list (MIT,
## CC0-1.0, Public-Domain; OFL-1.1 for fonts), or when a 3D art source carries a colour literal instead of the art
## palette tokens. Exit 1 on violations.

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const ArtRules := preload("res://tools/lib/art_rules.gd")


func _initialize() -> void:
	var violations: PackedStringArray = []
	var all: PackedStringArray = ToolFiles.list_files("res://", ArtRules.ASSET_EXTENSIONS + PackedStringArray(["res", "tres", "md", "txt"]))
	var assets: PackedStringArray = ArtRules.asset_files(all)
	var rows: Array[Dictionary] = ArtRules.parse_licenses(ToolFiles.read_text(ArtRules.LICENSES_PATH))
	if rows.is_empty():
		violations.append("%s is missing or has no licence rows" % ArtRules.LICENSES_PATH)
	violations.append_array(ArtRules.check_licenses(assets, rows))
	var sources: PackedStringArray = ToolFiles.list_files("res://", PackedStringArray(["tscn", "tres", "gd", "gdshader"]))
	var checked: int = assets.size()
	for path: String in sources:
		if ArtRules.in_palette_scope(path):
			checked += 1
			violations.append_array(ArtRules.check_palette(path, ToolFiles.read_text(path)))
	quit(ToolFiles.report("lint_art", violations, checked))
