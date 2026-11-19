extends GdUnitTestSuite
## F060-AC-07 (R32, DESIGN_SYSTEM §9, TC-UI-003): theme overrides in scenes fail the lint unless allowlisted; the
## project itself has none.

const ThemeRules := preload("res://tools/lib/theme_rules.gd")
const ToolFiles := preload("res://tools/lib/tool_files.gd")

const SCENE_WITH_OVERRIDES: String = """[gd_scene format=3]

[node name="Title" type="Label"]
theme_override_colors/font_color = Color(1, 0, 0, 1)
theme_override_font_sizes/font_size = 40
theme_override_constants/line_spacing = 4
"""


func test_color_and_font_size_overrides_fail() -> void:
	var violations: PackedStringArray = ThemeRules.check_scene("res://scenes/ui/x/X.tscn", SCENE_WITH_OVERRIDES, PackedStringArray())
	assert_int(violations.size()).is_equal(2)
	assert_str(violations[0]).contains("theme_override_colors/font_color")


func test_allowlisted_override_passes() -> void:
	var allow: PackedStringArray = [
		"res://scenes/ui/x/X.tscn::theme_override_colors/font_color",
		"res://scenes/ui/x/X.tscn::theme_override_font_sizes/font_size",
	]
	assert_int(ThemeRules.check_scene("res://scenes/ui/x/X.tscn", SCENE_WITH_OVERRIDES, allow).size()).is_equal(0)


func test_foreign_theme_resource_fails() -> void:
	var scene: String = "[gd_scene load_steps=2 format=3]\n[ext_resource type=\"Theme\" path=\"res://ui/my_theme.tres\" id=\"1\"]\n"
	assert_int(ThemeRules.check_scene("res://scenes/ui/x/X.tscn", scene, PackedStringArray()).size()).is_equal(1)
	var ok_scene: String = "[ext_resource type=\"Theme\" path=\"res://resources/ui/theme/eduworld_theme_g1_3.tres\" id=\"1\"]\n"
	assert_int(ThemeRules.check_scene("res://scenes/ui/x/X.tscn", ok_scene, PackedStringArray()).size()).is_equal(0)


func test_runtime_override_in_scene_script_fails() -> void:
	var source: String = "extends Label\nfunc _ready() -> void:\n\tadd_theme_color_override(\"font_color\", Color.RED)\n"
	assert_int(ThemeRules.check_script("res://scenes/ui/x/x.gd", source).size()).is_equal(1)


func test_non_generated_theme_resource_fails() -> void:
	assert_int(ThemeRules.check_theme_resource("res://resources/ui/other.tres").size()).is_equal(1)
	assert_int(ThemeRules.check_theme_resource("res://resources/ui/theme/eduworld_theme_g4_5.tres").size()).is_equal(0)


func test_project_scenes_have_no_override() -> void:
	var violations: PackedStringArray = []
	var allow: PackedStringArray = ThemeRules.load_allowlist()
	for path: String in ToolFiles.list_files("res://", PackedStringArray(["tscn"]), PackedStringArray(["res://tests"])):
		violations.append_array(ThemeRules.check_scene(path, ToolFiles.read_text(path), allow))
	assert_array(Array(violations)).is_empty()
	assert_array(Array(allow)).is_empty()
