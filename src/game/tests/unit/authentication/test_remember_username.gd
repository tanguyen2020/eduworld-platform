extends GdUnitTestSuite
## F061-AC-12 (TC-AUTH-045, ADR-0086, CONFIG_DEFAULTS `game_remember_username`): with "Nhớ tên đăng nhập" on and a
## personal device only `last_username` is written to `user://settings.cfg` and prefilled; the password field is
## always empty; on a `shared_device` the option is hidden and no username is stored.

const SettingsScript := preload("res://scripts/core/settings.gd")

var h: AuthHarness = null


func after_test() -> void:
	if h != null:
		h.dispose()
		h = null


func _settings_text() -> String:
	return FileAccess.get_file_as_string(AuthHarness.SETTINGS_PATH)


func test_personal_device_remembers_the_username_only() -> void:
	h = AuthHarness.create(false)
	assert_bool(h.auth.form.remember_visible).is_true()
	assert_bool(h.auth.form.remember_username).is_true()
	await h.sign_in()
	var cfg: ConfigFile = ConfigFile.new()
	assert_int(cfg.load(AuthHarness.SETTINGS_PATH)).is_equal(OK)
	assert_str(String(cfg.get_value("login", "last_username", ""))).is_equal("hs.an.3a")
	assert_str(_settings_text()).not_contains(AuthHarness.PASSWORD)
	assert_str(_settings_text()).not_contains(AuthHarness.REFRESH_1)
	var reopened: Node = auto_free(SettingsScript.new()) as Node
	reopened.call("load_from", AuthHarness.SETTINGS_PATH)
	var form: LoginFormModel = LoginFormModel.new()
	form.load_memory(bool(reopened.get("remember_username")), String(reopened.get("last_username")), false)
	assert_str(form.username).is_equal("hs.an.3a")
	await h.auth.logout()
	assert_str(h.auth.form.username).is_equal("hs.an.3a")


func test_unchecked_option_stores_nothing() -> void:
	h = AuthHarness.create(false)
	h.auth.set_remember_username(false)
	await h.sign_in()
	var cfg: ConfigFile = ConfigFile.new()
	cfg.load(AuthHarness.SETTINGS_PATH)
	assert_str(String(cfg.get_value("login", "last_username", ""))).is_empty()
	assert_bool(bool(cfg.get_value("login", "remember_username", true))).is_false()
	await h.auth.logout()
	assert_str(h.auth.form.username).is_empty()


func test_shared_device_hides_the_option_and_stores_nothing() -> void:
	h = AuthHarness.create(true)
	assert_bool(h.auth.form.remember_visible).is_false()
	assert_bool(h.auth.form.remember_username).is_false()
	await h.sign_in()
	assert_str(_settings_text()).not_contains("hs.an.3a")
	await h.auth.logout()
	assert_str(h.auth.form.username).is_empty()
