extends SceneTree
## CLI: `godot --headless --path src/game -s res://tools/check_typed.gd` (F060-AC-01, ADR-0014).
## The project turns `untyped_declaration` into an error, so any script with an untyped declaration (or another
## parse error) fails to load. Loads every project script outside addons/ and fails if one does not compile.

const ToolFiles := preload("res://tools/lib/tool_files.gd")


func _initialize() -> void:
	var violations: PackedStringArray = []
	var level: int = int(ProjectSettings.get_setting("debug/gdscript/warnings/untyped_declaration", 0))
	if level != 2:
		violations.append("project.godot: debug/gdscript/warnings/untyped_declaration must be 2 (error), got %d" % level)
	var files: PackedStringArray = ToolFiles.list_files("res://", PackedStringArray(["gd"]))
	for path: String in files:
		var script: GDScript = ResourceLoader.load(path, "GDScript", ResourceLoader.CACHE_MODE_REUSE) as GDScript
		if script == null or not script.can_instantiate():
			violations.append("%s does not compile (untyped declaration or parse error, see log above)" % path)
	quit(ToolFiles.report("check_typed", violations, files.size()))
