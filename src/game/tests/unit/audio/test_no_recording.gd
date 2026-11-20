extends GdUnitTestSuite
## F066-AC-20 (TC-AUD-004; T036): no microphone permission in the project or the export presets, no recording node in
## the learning scenes and scripts (ADR-0059, ADR-0072).

const PATTERNS: Array[String] = ["AudioEffectRecord", "AudioStreamMicrophone", "AudioEffectCapture", "record_audio=true"]


func _files(dir: String, out: PackedStringArray) -> void:
	var d: DirAccess = DirAccess.open(dir)
	if d == null:
		return
	for f: String in d.get_files():
		if f.ends_with(".gd") or f.ends_with(".tscn") or f.ends_with(".tres"):
			out.append(dir.path_join(f))
	for sub: String in d.get_directories():
		_files(dir.path_join(sub), out)


func test_no_microphone_and_no_recording_node() -> void:
	var files: PackedStringArray = ["res://project.godot", "res://export_presets.cfg"]
	_files("res://scenes/learning", files)
	_files("res://scripts/learning", files)
	assert_int(files.size()).is_greater(10)
	for path: String in files:
		if not FileAccess.file_exists(path):
			continue
		var text: String = FileAccess.get_file_as_string(path)
		for p: String in PATTERNS:
			assert_bool(text.contains(p)).override_failure_message("%s contains %s" % [path, p]).is_false()
	assert_bool(ProjectSettings.get_setting("audio/driver/enable_input", false)).is_false()
