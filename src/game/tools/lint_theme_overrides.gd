extends SceneTree
## CLI: `godot --headless --path src/game -s res://tools/lint_theme_overrides.gd` (F060-AC-07, TC-UI-003).
## Fails on theme overrides of colour/font/size/stylebox in .tscn (outside the allowlist), runtime overrides in
## scene scripts and Theme resources that are not generated from design/game-tokens.

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const ThemeRules := preload("res://tools/lib/theme_rules.gd")


func _initialize() -> void:
	var violations: PackedStringArray = []
	var allowlist: PackedStringArray = ThemeRules.load_allowlist()
	var files: PackedStringArray = ToolFiles.list_files("res://", PackedStringArray(["tscn", "gd", "tres"]), PackedStringArray(["res://tests", "res://tools"]))
	for path: String in files:
		var text: String = ToolFiles.read_text(path)
		match path.get_extension():
			"tscn":
				violations.append_array(ThemeRules.check_scene(path, text, allowlist))
			"gd":
				if path.begins_with("res://scenes/"):
					violations.append_array(ThemeRules.check_script(path, text))
			"tres":
				if text.contains("[gd_resource type=\"Theme\""):
					violations.append_array(ThemeRules.check_theme_resource(path))
	quit(ToolFiles.report("lint_theme_overrides", violations, files.size()))
