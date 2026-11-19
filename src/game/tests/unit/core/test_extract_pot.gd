extends GdUnitTestSuite
## F060-AC-13 (TC-I18N-005): keys collected by extract_pot must exist in vi.po and en.po; a new key without
## translation, a stale messages.pot or hard-coded scene text fail the check.

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const PoFile := preload("res://tools/lib/po_file.gd")
const I18nRules := preload("res://tools/lib/i18n_rules.gd")


func _project_keys() -> PackedStringArray:
	var keys: PackedStringArray = []
	for root_dir: String in I18nRules.SOURCE_ROOTS:
		for path: String in ToolFiles.list_files(root_dir, I18nRules.SOURCE_EXTENSIONS):
			for key: String in I18nRules.keys_in(ToolFiles.read_text(path)):
				if not key in keys:
					keys.append(key)
	return keys


func test_new_key_without_translation_is_reported() -> void:
	var keys: PackedStringArray = I18nRules.keys_in("func _ready() -> void:\n\tlabel.text = tr(\"ui.boot.new\")\n")
	assert_array(Array(keys)).is_equal(["ui.boot.new"])
	var vi: Dictionary = PoFile.parse(FileAccess.get_file_as_string("res://locale/vi.po"))
	assert_array(Array(I18nRules.missing(keys, vi))).is_equal(["ui.boot.new"])


func test_project_keys_are_all_translated() -> void:
	var keys: PackedStringArray = _project_keys()
	assert_int(keys.size()).is_greater(20)
	for catalog_path: String in ["res://locale/vi.po", "res://locale/en.po"]:
		var catalog: Dictionary = PoFile.parse(FileAccess.get_file_as_string(catalog_path))
		assert_array(Array(I18nRules.missing(keys, catalog))).override_failure_message(catalog_path).is_empty()


func test_messages_pot_lists_exactly_the_project_keys() -> void:
	var pot: Dictionary = PoFile.parse(FileAccess.get_file_as_string("res://locale/messages.pot"))
	var keys: Array = Array(_project_keys())
	assert_array(pot.keys()).contains_exactly_in_any_order(keys)


func test_hardcoded_scene_text_is_reported() -> void:
	var scene: String = "\n".join(PackedStringArray([
		"[node name=\"A\" type=\"Label\"]", "text = \"Xin chào\"",
		"[node name=\"B\" type=\"Label\"]", "text = \"42 %\"",
		"[node name=\"C\" type=\"Label\"]", "text = \"ui.hud.title\"",
	]))
	var found: PackedStringArray = I18nRules.hardcoded_text("res://scenes/x.tscn", scene)
	assert_int(found.size()).is_equal(1)
	assert_str(found[0]).contains("Xin chào")


func test_project_scenes_have_no_hardcoded_text() -> void:
	var found: PackedStringArray = []
	for path: String in ToolFiles.list_files("res://scenes", PackedStringArray(["tscn"])):
		found.append_array(I18nRules.hardcoded_text(path, ToolFiles.read_text(path)))
	assert_array(Array(found)).is_empty()


func test_po_round_trip_keeps_escapes() -> void:
	var entries: Dictionary = {"ui.a.b": "Say \"hi\"\nnext \\ line"}
	var text: String = PoFile.write(PackedStringArray(["Language: vi"]), entries)
	assert_dict(PoFile.parse(text)).is_equal(entries)
