extends GdUnitTestSuite
## F133 T007 (F133-AC-01, F133-AC-02, AGENTS R3/R4): Config reads the file written by scripts/godot/gen-config.mjs
## (fixture config_production.json: client_version 1.2.3 from the tag, build_number = run number 77, store links,
## bundle/release CDN, manifest public key) and a release build never merges `user://config_override.json`.

const ConfigScript := preload("res://scripts/core/config.gd")
const FIXTURE: String = "res://tests/fixtures/release/config_production.json"

var _config: Node = null


func before_test() -> void:
	_config = auto_free(ConfigScript.new()) as Node


func after_test() -> void:
	if FileAccess.file_exists(ConfigScript.OVERRIDE_PATH):
		DirAccess.remove_absolute(ConfigScript.OVERRIDE_PATH)


func test_generated_file_carries_version_build_and_release_keys() -> void:
	assert_int(_config.call("load_file", FIXTURE, false)).override_failure_message("%s %s" % [
		_config.get("error_code"), _config.get("error_key")]).is_equal(OK)
	assert_str(String(_config.get("client_version"))).is_equal("1.2.3")
	assert_int(int(_config.get("build_number"))).is_equal(77)
	assert_str(String(_config.get("bundles_base_url"))).is_equal("https://bundles.eduworld.example")
	assert_str(String(_config.get("releases_base_url"))).is_equal("https://play.eduworld.example/releases")
	var stores: Dictionary = _config.get("store_urls") as Dictionary
	assert_str(String(stores.get("android", ""))).starts_with("https://play.google.com/")
	assert_str(String(stores.get("ios", ""))).starts_with("https://apps.apple.com/")
	assert_str(String(_config.get("manifest_public_key"))).starts_with("-----BEGIN PUBLIC KEY-----")


func test_release_build_ignores_the_user_override() -> void:
	var f: FileAccess = FileAccess.open(ConfigScript.OVERRIDE_PATH, FileAccess.WRITE)
	f.store_string(JSON.stringify({"api_base_url": "https://evil.example/api/v1"}))
	f.close()
	assert_bool(ConfigScript.override_allowed(false)).is_false()
	assert_bool(ConfigScript.override_allowed(true)).is_true()
	_config.call("load_file", FIXTURE, ConfigScript.override_allowed(false))
	assert_str(String(_config.get("api_base_url"))).is_equal("https://api.eduworld.example/api/v1")
	_config.call("load_file", FIXTURE, true)
	assert_str(String(_config.get("api_base_url"))).is_equal("https://evil.example/api/v1")


func test_release_keys_are_type_checked_and_a_private_key_is_refused() -> void:
	var base: Dictionary = JSON.parse_string(FileAccess.get_file_as_string(FIXTURE)) as Dictionary
	var bad: Dictionary = base.duplicate(true)
	bad["build_number"] = "77"
	assert_int(_config.call("validate", bad)).is_equal(ERR_INVALID_DATA)
	assert_str(String(_config.get("error_key"))).is_equal("build_number")
	bad = base.duplicate(true)
	bad["store_urls"] = {"android": "http://insecure.example"}
	assert_int(_config.call("validate", bad)).is_equal(ERR_INVALID_DATA)
	assert_str(String(_config.get("error_key"))).is_equal("store_urls")
	bad = base.duplicate(true)
	bad["manifest_public_key"] = "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"
	assert_int(_config.call("validate", bad)).is_equal(ERR_INVALID_DATA)
	assert_str(String(_config.get("error_code"))).is_equal(ConfigScript.ERR_KEY_FORBIDDEN)


func test_committed_production_file_has_no_build_number_and_loads() -> void:
	assert_int(_config.call("load_env", "production", false)).is_equal(OK)
	assert_int(int(_config.get("build_number"))).is_equal(0)
