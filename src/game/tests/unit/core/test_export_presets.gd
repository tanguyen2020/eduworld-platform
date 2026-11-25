extends GdUnitTestSuite
## F133 T002 (F133-AC-01, F133-AC-02, F133-AC-16, ADR-0101, ADR-0086, ADR-0068): the release export presets are
## exactly Android, iOS and Web; all three pack only the production configuration with the same exclusions; no
## password, certificate, team id or provisioning profile is committed; Android arm64-v8a from Android 9, iOS arm64
## from iOS 15 without extra entitlements, Web Compatibility single-threaded with the EduWorld shell.

const EXPORT_PRESETS: String = "res://export_presets.cfg"
const EXCLUDED: Array[String] = [
	"config/env/mock.json", "config/env/local.json", "config/env/dev.json", "config/env/staging.json", "tests/*",
	"tools/*", "scenes/ui/debug/*", "locale/xx_PS.po", "locale/messages.pot", "addons/gdUnit4/*", "reports/*",
	"web/*",
]
## Option name fragments that hold signing material; their values must stay empty in the repository (CI writes them).
const SECRET_OPTIONS: Array[String] = [
	"password", "keystore/", "app_store_team_id", "code_sign_identity", "provisioning_profile",
]

var _cfg: ConfigFile


func before() -> void:
	_cfg = ConfigFile.new()
	assert_int(_cfg.load(EXPORT_PRESETS)).is_equal(OK)


func _presets() -> Dictionary:
	var out: Dictionary = {}
	for section: String in _cfg.get_sections():
		if section.begins_with("preset.") and not section.ends_with(".options"):
			out[String(_cfg.get_value(section, "platform", ""))] = section
	return out


func _option(platform: String, key: String) -> Variant:
	return _cfg.get_value("%s.options" % _presets()[platform], key, null)


func test_exactly_android_ios_and_web() -> void:
	var platforms: Array = _presets().keys()
	platforms.sort()
	assert_array(platforms).is_equal(["Android", "Web", "iOS"])
	for section: String in _presets().values():
		assert_str(String(_cfg.get_value(section, "name", ""))).is_not_empty()
		assert_str(String(_cfg.get_value(section, "platform", ""))).not_contains("Windows")


func test_every_preset_packs_only_the_production_configuration() -> void:
	for platform: String in _presets():
		var section: String = _presets()[platform]
		assert_str(String(_cfg.get_value(section, "custom_features", ""))) \
			.override_failure_message(platform).is_equal("eduworld_production")
		assert_str(String(_cfg.get_value(section, "include_filter", ""))) \
			.override_failure_message(platform).is_equal("config/env/production.json")
		var exclude: PackedStringArray = String(_cfg.get_value(section, "exclude_filter", "")).split(",")
		var entries: Array[String] = []
		for entry: String in exclude:
			entries.append(entry.strip_edges())
		for path: String in EXCLUDED:
			assert_bool(entries.has(path)).override_failure_message("%s does not exclude %s" % [platform, path]) \
				.is_true()
		assert_bool(bool(_cfg.get_value(section, "encrypt_pck", true))).is_false()


func test_no_signing_material_is_committed() -> void:
	for platform: String in _presets():
		var options: String = "%s.options" % _presets()[platform]
		for key: String in _cfg.get_section_keys(options):
			for fragment: String in SECRET_OPTIONS:
				if key.contains(fragment):
					assert_str(str(_cfg.get_value(options, key))) \
						.override_failure_message("%s %s must be empty" % [platform, key]).is_empty()


func test_android_arm64_from_android_9_without_backup_or_login_intent() -> void:
	assert_bool(bool(_option("Android", "architectures/arm64-v8a"))).is_true()
	for arch: String in ["armeabi-v7a", "x86", "x86_64"]:
		assert_bool(bool(_option("Android", "architectures/%s" % arch))).is_false()
	assert_str(str(_option("Android", "gradle_build/min_sdk"))).is_equal("28")
	assert_str(str(_option("Android", "package/unique_name"))).is_equal("vn.eduworld.game")
	assert_bool(bool(_option("Android", "user_data_backup/allow"))).is_false()
	assert_array(_option("Android", "permissions/custom_permissions") as PackedStringArray).is_empty()


func test_ios_arm64_from_ios_15_without_extra_entitlements() -> void:
	assert_str(str(_option("iOS", "application/bundle_identifier"))).is_equal("vn.eduworld.game")
	assert_str(str(_option("iOS", "application/min_ios_version"))).is_equal("15.0")
	assert_int(int(_option("iOS", "application/export_method_release"))).is_equal(0)
	# ADR-0086 / F133-AC-23: no Keychain sharing, no iCloud, no extra keychain-access-groups.
	assert_str(str(_option("iOS", "entitlements/additional"))).is_empty()
	assert_str(str(_option("iOS", "application/additional_plist_content"))).not_contains("keychain")
	assert_bool(bool(_option("iOS", "entitlements/game_center"))).is_false()
	assert_bool(bool(_option("iOS", "user_data/accessible_from_files_app"))).is_false()
	assert_bool(bool(_option("iOS", "user_data/accessible_from_itunes_sharing"))).is_false()


func test_web_is_single_threaded_compatibility_with_the_eduworld_shell() -> void:
	assert_bool(bool(_option("Web", "variant/thread_support"))).is_false()
	assert_bool(bool(_option("Web", "variant/extensions_support"))).is_false()
	assert_bool(bool(_option("Web", "progressive_web_app/ensure_cross_origin_isolation_headers"))).is_false()
	assert_bool(bool(_option("Web", "vram_texture_compression/for_mobile"))).is_true()
	var shell: String = str(_option("Web", "html/custom_html_shell"))
	assert_str(shell).is_equal("res://web/shell.html")
	assert_bool(FileAccess.file_exists(shell)).is_true()
	assert_str(String(_cfg.get_value(_presets()["Web"], "export_path", ""))).is_equal("build/web/index.html")
	assert_str(str(ProjectSettings.get_setting("rendering/renderer/rendering_method.web"))) \
		.is_equal("gl_compatibility")
