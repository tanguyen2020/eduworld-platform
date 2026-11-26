extends GdUnitTestSuite
## T014 (F119-AC-10, AC-11; AC-UX-106, TC-CHAT-015; AGENTS R15): ChatFeedbackMapper turns every server decision of
## CHAT_SYSTEM §4.3 into its friendly sentence, never repeats anything of the message (it does not even receive it), and
## every sentence exists in vi and en.

const PoFile := preload("res://tools/lib/po_file.gd")


func test_moderation_decisions_map_to_the_table_of_chat_system_4_3() -> void:
	assert_str(String(ChatFeedbackMapper.for_masked()["key"])).is_equal("ui.chat.feedback.masked")
	assert_str(String(ChatFeedbackMapper.for_code("CHAT_CONTENT_BLOCKED", "pii")["key"])).is_equal("ui.chat.feedback.pii_blocked")
	assert_str(String(ChatFeedbackMapper.for_code("CHAT_CONTENT_BLOCKED", "pii_pattern")["key"])).is_equal("ui.chat.feedback.pii_blocked")
	assert_str(String(ChatFeedbackMapper.for_code("CHAT_CONTENT_BLOCKED", "insult")["key"])).is_equal("ui.chat.feedback.term_blocked")
	assert_str(String(ChatFeedbackMapper.for_code("CHAT_CONTENT_BLOCKED", "")["key"])).is_equal("ui.chat.feedback.term_blocked")
	assert_str(String(ChatFeedbackMapper.for_held()["key"])).is_equal("ui.chat.feedback.held")
	var care: Dictionary = ChatFeedbackMapper.for_code("CHAT_CONTENT_BLOCKED", "grooming_scam")
	assert_str(String(care["key"])).is_equal("ui.chat.feedback.care")
	assert_str(String(care["tone"])).is_equal(ChatFeedbackMapper.TONE_CARE)
	assert_str(String(ChatFeedbackMapper.for_code("CHAT_RATE_LIMITED")["key"])).is_equal("ui.chat.feedback.rate_limited")


func test_sanctions_lock_with_time_or_care() -> void:
	var muted: Dictionary = ChatFeedbackMapper.for_sanction("mute", "auto_spam", "10:30")
	assert_str(String(muted["key"])).is_equal("ui.chat.muted_until")
	assert_dict(muted["args"] as Dictionary).is_equal({"time": "10:30"})
	assert_str(String(ChatFeedbackMapper.for_sanction("mute", "", "")["key"])).is_equal("ui.chat.feedback.muted")
	assert_str(String(ChatFeedbackMapper.for_sanction("mute", "block_and_alert_hold", "10:30")["key"])).is_equal("ui.chat.feedback.care")
	assert_str(String(ChatFeedbackMapper.for_sanction("restrict_text", "", "")["key"])).is_equal("ui.chat.feedback.restrict_text")


func test_unknown_codes_show_the_generic_sentence_never_the_code() -> void:
	var fb: Dictionary = ChatFeedbackMapper.for_code("SOMETHING_NEW")
	assert_str(String(fb["key"])).is_equal("ui.chat.feedback.generic")


func test_output_never_carries_matched_words_or_positions() -> void:
	for code: String in ChatFeedbackMapper.CODE_KEYS.keys():
		var fb: Dictionary = ChatFeedbackMapper.for_code(code, "pii")
		assert_array((fb["args"] as Dictionary).keys()).is_empty()
	var blocked: Dictionary = ChatFeedbackMapper.for_code("CHAT_CONTENT_BLOCKED", "insult")
	assert_array(blocked.keys()).is_equal(["key", "args", "tone"])
	assert_array((blocked["args"] as Dictionary).keys()).is_empty()


func test_every_feedback_sentence_exists_in_vi_and_en() -> void:
	for po: String in ["res://locale/vi.po", "res://locale/en.po"]:
		var entries: Dictionary = PoFile.parse(FileAccess.get_file_as_string(po))
		for key: String in ChatFeedbackMapper.all_keys():
			assert_str(String(entries.get(key, ""))).override_failure_message("%s missing %s" % [po, key]).is_not_empty()
