extends GdUnitTestSuite
## F060-AC-03 (ADR-0068, AGENTS R3, R4, TC-AUTH-010): Config reads res://config/env/<env>.json chosen by --env,
## rejects secrets and OIDC keys, reports a missing key; every committed env file is valid and secret-free.

const ConfigScript := preload("res://scripts/core/config.gd")
const ENVS: PackedStringArray = ["local", "dev", "staging", "production", "mock"]

var _config: Node = null


func before_test() -> void:
	_config = auto_free(ConfigScript.new()) as Node


func _valid() -> Dictionary:
	return {
		"env": "local", "api_base_url": "http://localhost:9080/api/v1", "realtime_url": "ws://localhost:9080/realtime/v1",
		"cdn_base_url": "http://localhost:8090", "login_path": "/auth/game/login", "client_version": "1.0.0",
	}


func test_every_committed_env_file_loads() -> void:
	for env: String in ENVS:
		assert_int(_config.call("load_env", env, false)).override_failure_message("env %s: %s %s" % [
			env, _config.get("error_code"), _config.get("error_key")]).is_equal(OK)
		assert_str(String(_config.get("env_name"))).is_equal(env)


func test_only_mock_env_is_mock() -> void:
	for env: String in ENVS:
		_config.call("load_env", env, false)
		assert_bool(bool(_config.get("is_mock"))).is_equal(env == "mock")


func test_env_comes_from_cmdline_in_debug_only() -> void:
	var args: PackedStringArray = ["--foo", "--env=mock"]
	assert_str(ConfigScript.resolve_env_name(args, true)).is_equal("mock")
	assert_str(ConfigScript.resolve_env_name(args, false)).is_equal(
		String(ProjectSettings.get_setting_with_override("eduworld/config/env")))


func test_missing_required_key_is_rejected() -> void:
	var data: Dictionary = _valid()
	data.erase("api_base_url")
	assert_int(_config.call("validate", data)).is_equal(ERR_INVALID_DATA)
	assert_str(String(_config.get("error_code"))).is_equal(ConfigScript.ERR_KEY_MISSING)
	assert_str(String(_config.get("error_key"))).is_equal("api_base_url")


func test_oidc_and_secret_keys_are_rejected() -> void:
	for forbidden: String in ["oidc_issuer", "oidc_client_id", "client_secret", "api_key", "refresh_token"]:
		var data: Dictionary = _valid()
		data[forbidden] = "x"
		assert_int(_config.call("validate", data)).is_equal(ERR_INVALID_DATA)
		assert_str(String(_config.get("error_code"))).is_equal(ConfigScript.ERR_KEY_FORBIDDEN)


func test_keycloak_token_endpoint_value_is_rejected() -> void:
	var data: Dictionary = _valid()
	data["api_base_url"] = "https://auth.example/realms/eduworld/protocol/openid-connect/token"
	assert_int(_config.call("validate", data)).is_equal(ERR_INVALID_DATA)


func test_unknown_env_and_bad_name_fail() -> void:
	assert_int(_config.call("load_env", "nope", false)).is_equal(ERR_INVALID_DATA)
	assert_str(String(_config.get("error_code"))).is_equal(ConfigScript.ERR_FILE_MISSING)
	assert_int(_config.call("load_env", "../etc", false)).is_equal(ERR_INVALID_DATA)
	assert_str(String(_config.get("error_code"))).is_equal(ConfigScript.ERR_ENV_INVALID)
	assert_bool(bool(_config.call("is_valid"))).is_false()


func test_committed_env_files_have_no_secret_text() -> void:
	for env: String in ENVS:
		var text: String = FileAccess.get_file_as_string(ConfigScript.env_path(env)).to_lower()
		for needle: String in ["secret", "password", "oidc", "openid-connect", "client_id"]:
			assert_bool(text.contains(needle)).override_failure_message("%s contains %s" % [env, needle]).is_false()
