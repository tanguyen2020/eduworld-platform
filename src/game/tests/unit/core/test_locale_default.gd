extends GdUnitTestSuite
## F060-AC-15 (NFR-I18N-001, ADR-0056): locale only from `supported_locales`; vi-VN before bootstrap; after
## bootstrap the student's profile language, then `tenant_default_locale`.

const SettingsScript := preload("res://scripts/core/settings.gd")
const TEST_PATH: String = "user://test_locale_settings.cfg"


func after_test() -> void:
	DirAccess.remove_absolute(TEST_PATH)


func test_supported_locales_come_from_project_settings() -> void:
	assert_array(Array(L10n.supported_locales())).is_equal(["vi-VN", "en-US"])
	assert_str(L10n.default_locale()).is_equal("vi-VN")


func test_before_bootstrap() -> void:
	assert_str(L10n.resolve_locale("")).is_equal("vi-VN")
	assert_str(L10n.resolve_locale("fr-FR")).is_equal("vi-VN")
	assert_str(L10n.resolve_locale("en-US")).is_equal("en-US")


func test_after_bootstrap_profile_then_tenant() -> void:
	assert_str(L10n.resolve_locale("vi-VN", true, "en-US", "vi-VN")).is_equal("en-US")
	assert_str(L10n.resolve_locale("en-US", true, "", "vi-VN")).is_equal("vi-VN")
	assert_str(L10n.resolve_locale("en-US", true, "fr-FR", "de-DE")).is_equal("vi-VN")


func test_unsupported_locale_in_settings_file_is_ignored() -> void:
	var cfg: ConfigFile = ConfigFile.new()
	cfg.set_value("ui", "locale", "fr-FR")
	cfg.save(TEST_PATH)
	var settings: Node = auto_free(SettingsScript.new()) as Node
	settings.call("load_from", TEST_PATH)
	assert_str(String(settings.get("locale"))).is_equal("vi-VN")


func test_set_locale_rejects_unsupported() -> void:
	var before: String = String(Settings.locale)
	assert_bool(Settings.set_locale("fr-FR", false)).is_false()
	assert_str(String(Settings.locale)).is_equal(before)


func test_pseudo_locale_only_in_debug_builds() -> void:
	assert_bool(L10n.is_supported("xx-PS", true)).is_true()
	assert_bool(L10n.is_supported("xx-PS", false)).is_false()
