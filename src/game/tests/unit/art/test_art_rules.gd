extends GdUnitTestSuite
## F067-AC-03 (ADR-0102): every graphic asset has a row in res://assets/LICENSES.md with an allowed licence (MIT,
## CC0-1.0, Public-Domain; OFL-1.1 only for fonts); share-alike, non-commercial, no-derivatives, GPL and paid/store
## licences fail, and so does a file without a row. F067-AC-01/AC-04 (R32): 3D art sources carry no colour literal.

const ArtRules := preload("res://tools/lib/art_rules.gd")
const ToolFiles := preload("res://tools/lib/tool_files.gd")

const TABLE: String = """
| Path (glob) | Source | Author | License | Date |
|---|---|---|---|---|
| `assets/models/**` | First-party | EduWorld | MIT | 2026-11-24 |
| `assets/textures/cc0/*.png` | https://example.org/pack | Someone | CC0-1.0 | 2026-11-24 |
| `assets/textures/sa/*.png` | https://example.org/sa | Someone | CC-BY-SA-4.0 | 2026-11-24 |
| `assets/textures/nc/*.png` | https://example.org/nc | Someone | CC-BY-NC-4.0 | 2026-11-24 |
| `assets/audio/gpl/*.ogg` | https://example.org/gpl | Someone | GPL-3.0 | 2026-11-24 |
| `assets/store/*.glb` | https://store.example.org | Vendor | Asset Store EULA | 2026-11-24 |
| `resources/ui/theme/fonts/*` | https://example.org/font | Font authors | OFL-1.1 | 2026-11-19 |
| `assets/icons/*.svg` | https://example.org/ofl | Someone | OFL-1.1 | 2026-11-24 |
"""


func _rows() -> Array[Dictionary]:
	return ArtRules.parse_licenses(TABLE)


func test_allowed_licences_pass() -> void:
	for license: String in ["MIT", "CC0-1.0", "Public-Domain"]:
		assert_str(ArtRules.license_problem(license, "png")).override_failure_message(license).is_empty()
	assert_str(ArtRules.license_problem("OFL-1.1", "ttf")).is_empty()


func test_forbidden_licences_fail() -> void:
	for license: String in ["CC-BY-SA-4.0", "CC-BY-NC-4.0", "CC-BY-ND-4.0", "GPL-3.0", "LGPL-2.1", "AGPL-3.0",
			"Unity Asset Store EULA", "Proprietary", "CC-BY-NC-SA-3.0"]:
		assert_str(ArtRules.license_problem(license, "png")).override_failure_message(license).contains("forbidden")


func test_unknown_licence_and_ofl_on_graphics_fail() -> void:
	assert_str(ArtRules.license_problem("CC-BY-4.0", "png")).contains("allow list")
	assert_str(ArtRules.license_problem("OFL-1.1", "svg")).contains("fonts only")


func test_files_are_matched_to_rows_by_glob() -> void:
	var files: PackedStringArray = [
		"res://assets/models/env/env_hub_terrain_a.res", "res://assets/textures/cc0/grass.png",
		"res://resources/ui/theme/fonts/Baloo2-Bold.ttf",
	]
	assert_array(ArtRules.check_licenses(files, _rows())).is_empty()


func test_missing_row_and_forbidden_rows_are_reported() -> void:
	var files: PackedStringArray = [
		"res://assets/textures/unlisted/rock.png", "res://assets/textures/sa/a.png", "res://assets/textures/nc/b.png",
		"res://assets/audio/gpl/c.ogg", "res://assets/store/d.glb", "res://assets/icons/e.svg",
	]
	var out: PackedStringArray = ArtRules.check_licenses(files, _rows())
	assert_int(out.size()).is_equal(6)
	assert_str(out[0]).contains("has no licence row")


func test_rows_need_source_author_licence_and_date() -> void:
	var rows: Array[Dictionary] = ArtRules.parse_licenses("| `assets/x/*.png` |  | Someone | MIT |  |")
	var out: PackedStringArray = ArtRules.check_licenses(PackedStringArray(), rows)
	assert_int(out.size()).is_equal(2)


func test_asset_files_cover_assets_dir_and_media_outside_tests() -> void:
	var files: PackedStringArray = ArtRules.asset_files(PackedStringArray([
		"res://assets/models/a.res", "res://assets/LICENSES.md", "res://resources/x.png", "res://tests/y.png",
		"res://addons/z.svg", "res://scripts/a.gd", "res://resources/art/shaders/toon.gdshader",
	]))
	assert_array(files).contains_exactly(["res://assets/models/a.res", "res://resources/x.png",
		"res://resources/art/shaders/toon.gdshader"])


## F067-AC-03: the project's own LICENSES.md covers every asset file with an allowed licence.
func test_project_assets_all_have_allowed_licences() -> void:
	var all: PackedStringArray = ToolFiles.list_files("res://", ArtRules.ASSET_EXTENSIONS + PackedStringArray(["res", "tres", "md", "txt"]))
	var assets: PackedStringArray = ArtRules.asset_files(all)
	assert_int(assets.size()).is_greater(40)
	var rows: Array[Dictionary] = ArtRules.parse_licenses(ToolFiles.read_text(ArtRules.LICENSES_PATH))
	assert_array(ArtRules.check_licenses(assets, rows)).is_empty()


func test_palette_rule_flags_colour_literals() -> void:
	var tscn: String = "[node name=\"A\" type=\"MeshInstance3D\"]\nmodulate = Color(1, 0, 0, 1)\n"
	assert_int(ArtRules.check_palette("res://scenes/world/X.tscn", tscn).size()).is_equal(1)
	var gd: String = "var a: Color = Color8(255, 0, 0)\nvar b: String = \"#FF00AA\"\n# Color(1, 1, 1) in a comment\n"
	assert_int(ArtRules.check_palette("res://tools/art/x.gd", gd).size()).is_equal(2)
	var marked: String = "c = Color(n.x, n.y, n.z, 1.0)  # art-lint: not a colour\n"
	assert_array(ArtRules.check_palette("res://tools/art/x.gd", marked)).is_empty()
	assert_bool(ArtRules.in_palette_scope("res://resources/art/generated/mat_toon.tres")).is_false()
	assert_bool(ArtRules.in_palette_scope("res://scenes/world/zones/main_hub/Zone.tscn")).is_true()


## F067-AC-01, AC-04: no colour literal in the hub, the characters, the baked-art recipes or the shaders.
func test_project_art_sources_have_no_colour_literal() -> void:
	var violations: PackedStringArray = []
	var checked: int = 0
	for path: String in ToolFiles.list_files("res://", PackedStringArray(["tscn", "tres", "gd", "gdshader"])):
		if ArtRules.in_palette_scope(path):
			checked += 1
			violations.append_array(ArtRules.check_palette(path, ToolFiles.read_text(path)))
	assert_int(checked).is_greater(30)
	assert_array(violations).is_empty()
