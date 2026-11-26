extends GdUnitTestSuite
## T027, T028 (F119-AC-22, AC-23; TC-I18N-006, AGENTS R1, R14, R15, R25, R32): the chat code talks only to the realtime
## WebSocket and the REST API (no NATS, Valkey, raw socket or third-party GIF host), holds no word list or moderation
## rule and no switch that turns the gameplay chat off, styles itself only through the generated Theme (no theme
## override, no colour literal), takes every visible word from a `ui.chat.*` / shared key present in vi and en, and logs
## no message content.

const ToolFiles := preload("res://tools/lib/tool_files.gd")
const PoFile := preload("res://tools/lib/po_file.gd")
const I18nRules := preload("res://tools/lib/i18n_rules.gd")
const ROOTS: PackedStringArray = ["res://scripts/chat", "res://scenes/chat", "res://scripts/social"]
## Words of infrastructure the game client must never reach (CHAT_SYSTEM §5.1, R25).
const INFRA: PackedStringArray = ["nats", "valkey", "redis", "jetstream", "ew:chat", "giphy", "tenor", "StreamPeerTCP",
	"PacketPeerUDP", "WebSocketPeer"]
## Names a "turn the chat off" option would have (ADR-0020 §6: always-on).
const OFF_SWITCHES: PackedStringArray = ["chat_enabled", "disable_chat", "chat_off", "toggle_chat", "hide_chat_forever",
	"mute_channel", "chat_disabled"]
## Moderation logic markers (word lists, matching) that belong to the server only.
const MODERATION: PackedStringArray = ["sensitive_term", "banned_word", "profanity", "regex_filter", "blocklist", "censor"]


func _sources() -> PackedStringArray:
	var out: PackedStringArray = []
	for root: String in ROOTS:
		out.append_array(ToolFiles.list_files(root, PackedStringArray(["gd", "tscn"])))
	return out


func _code_lines(path: String) -> PackedStringArray:
	var out: PackedStringArray = []
	for line: String in ToolFiles.read_text(path).split("\n"):
		var code: String = ToolFiles.strip_comment(line).strip_edges()
		if not code.begins_with("##") and not code.is_empty():
			out.append(code)
	return out


func test_no_connection_to_internal_infrastructure_or_third_party_gif() -> void:
	var hits: PackedStringArray = []
	for path: String in _sources():
		for code: String in _code_lines(path):
			for word: String in INFRA:
				if code.to_lower().contains(word.to_lower()):
					hits.append("%s: %s" % [path, word])
	assert_array(hits).override_failure_message("\n".join(hits)).is_empty()


func test_no_switch_turns_the_gameplay_chat_off() -> void:
	var hits: PackedStringArray = []
	for path: String in _sources():
		for code: String in _code_lines(path):
			for word: String in OFF_SWITCHES:
				if code.to_lower().contains(word):
					hits.append("%s: %s" % [path, word])
	assert_array(hits).override_failure_message("\n".join(hits)).is_empty()


func test_no_word_list_or_moderation_rule_in_the_client() -> void:
	var hits: PackedStringArray = []
	for path: String in _sources():
		for code: String in _code_lines(path):
			for word: String in MODERATION:
				if code.to_lower().contains(word):
					hits.append("%s: %s" % [path, word])
			if code.contains("RegEx"):
				hits.append("%s: RegEx" % path)
	assert_array(hits).override_failure_message("\n".join(hits)).is_empty()


func test_style_only_through_the_theme() -> void:
	var hits: PackedStringArray = []
	for path: String in _sources():
		for code: String in _code_lines(path):
			if code.contains("add_theme_") or code.contains("Color(") or code.contains("Color8(") or code.contains("Color.html"):
				hits.append("%s: %s" % [path, code.left(80)])
	assert_array(hits).override_failure_message("\n".join(hits)).is_empty()


func test_every_chat_key_is_translated_in_vi_and_en() -> void:
	var keys: PackedStringArray = []
	for path: String in _sources():
		for key: String in I18nRules.keys_in(ToolFiles.read_text(path)):
			if not key in keys:
				keys.append(key)
	assert_int(keys.size()).is_greater(100)
	for po: String in ["res://locale/vi.po", "res://locale/en.po"]:
		var entries: Dictionary = PoFile.parse(FileAccess.get_file_as_string(po))
		var missing: PackedStringArray = []
		for key: String in keys:
			if String(entries.get(key, "")).is_empty():
				missing.append(key)
		assert_array(missing).override_failure_message("%s: %s" % [po, ", ".join(missing)]).is_empty()


func test_chat_logs_carry_codes_never_content() -> void:
	var hits: PackedStringArray = []
	for path: String in _sources():
		for code: String in _code_lines(path):
			if code.contains("Log.") and (code.contains("body") or code.contains("text") or code.contains("nickname")):
				hits.append("%s: %s" % [path, code.left(100)])
	assert_array(hits).override_failure_message("\n".join(hits)).is_empty()
	assert_str(String(Log.redact("x", "body"))).is_equal("[REDACTED]")
