extends SceneTree
## CLI: `godot --headless --path src/game -s res://tools/lint_signals.gd` (F060-AC-06). Exit 1 on violations.

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const SignalRules := preload("res://tools/lib/signal_rules.gd")


func _initialize() -> void:
	var violations: PackedStringArray = []
	var files: PackedStringArray = ToolFiles.list_files("res://", PackedStringArray(["gd"]), PackedStringArray(["res://tests"]))
	for path: String in files:
		violations.append_array(SignalRules.check(path, ToolFiles.read_text(path)))
	quit(ToolFiles.report("lint_signals", violations, files.size()))
