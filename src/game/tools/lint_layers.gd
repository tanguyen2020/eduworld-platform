extends SceneTree
## CLI: `godot --headless --path src/game -s res://tools/lint_layers.gd` (F060-AC-05). Exit 1 on violations.

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const LayerRules := preload("res://tools/lib/layer_rules.gd")


func _initialize() -> void:
	var violations: PackedStringArray = []
	var files: PackedStringArray = ToolFiles.list_files("res://", PackedStringArray(["gd", "tscn"]))
	var checked: int = 0
	for path: String in files:
		if LayerRules.applies_to(path):
			checked += 1
			violations.append_array(LayerRules.check(path, ToolFiles.read_text(path)))
	quit(ToolFiles.report("lint_layers", violations, checked))
