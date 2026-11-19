extends GdUnitTestSuite
## F060-AC-04 (AGENTS R14): structured log into a ring buffer; tokens, passwords, Authorization and real names are
## redacted before storage.

const LoggerScript := preload("res://scripts/core/logger.gd")

var _log: Node = null


func before_test() -> void:
	_log = auto_free(LoggerScript.new()) as Node
	_log.set("min_level", 0)


func _last_fields() -> Dictionary:
	var entries: Array[Dictionary] = _log.call("entries") as Array[Dictionary]
	return entries[entries.size() - 1]["fields"] as Dictionary


func test_sensitive_fields_are_redacted() -> void:
	_log.call("info", "login_attempted", {
		"password": "p@ss", "access_token": "tok-a", "refresh_token": "tok-r", "Authorization": "Bearer abc.def",
		"full_name": "Nguyen Van An", "username": "hs.an.3a", "nested": {"client_secret": "s", "ok": 1},
	})
	var fields: Dictionary = _last_fields()
	for key: String in ["password", "access_token", "refresh_token", "Authorization", "full_name"]:
		assert_str(String(fields[key])).is_equal(LoggerScript.REDACTED)
	assert_str(String((fields["nested"] as Dictionary)["client_secret"])).is_equal(LoggerScript.REDACTED)
	assert_int(int((fields["nested"] as Dictionary)["ok"])).is_equal(1)
	assert_str(String(fields["username"])).is_equal("hs.an.3a")
	var dump: String = JSON.stringify(_log.call("entries"))
	for secret: String in ["p@ss", "tok-a", "tok-r", "abc.def", "Nguyen Van An"]:
		assert_bool(dump.contains(secret)).is_false()


func test_bearer_and_jwt_values_are_redacted_under_any_key() -> void:
	var jwt: String = "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiIxMjM0NTYifQ.c2lnbmF0dXJl"
	_log.call("warn", "request_failed", {"detail": "header Bearer xyz123 rejected", "echo": jwt})
	var fields: Dictionary = _last_fields()
	assert_bool(String(fields["detail"]).contains("xyz123")).is_false()
	assert_bool(String(fields["echo"]).contains(jwt)).is_false()


func test_ring_buffer_keeps_the_last_entries() -> void:
	var capacity: int = int(_log.call("capacity"))
	assert_int(capacity).is_equal(int(ProjectSettings.get_setting("eduworld/log/ring_buffer_size")))
	for i: int in capacity + 10:
		_log.call("debug", "tick", {"i": i})
	var entries: Array[Dictionary] = _log.call("entries") as Array[Dictionary]
	assert_int(entries.size()).is_equal(capacity)
	assert_int(int((entries[0]["fields"] as Dictionary)["i"])).is_equal(10)


func test_entries_below_level_are_dropped() -> void:
	_log.call("set_level_name", "warn")
	_log.call("info", "noise")
	assert_int((_log.call("entries") as Array[Dictionary]).size()).is_equal(0)
	_log.call("error", "boom")
	assert_str(String((_log.call("entries") as Array[Dictionary])[0]["level"])).is_equal("error")
