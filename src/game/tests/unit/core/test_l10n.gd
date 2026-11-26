extends GdUnitTestSuite
## F060-AC-12 (NFR-I18N-001, ADR-0056): API error codes map to `ui.error.<CODE>`; an unknown code shows
## `ui.error.generic` (never the raw key) and queues the telemetry event `i18n.missing_key`.

const PoFile := preload("res://tools/lib/po_file.gd")

var _locale: String = ""


func before_test() -> void:
	_locale = TranslationServer.get_locale()
	TranslationServer.set_locale("vi-VN")
	AppState.pending_telemetry.clear()


func after_test() -> void:
	TranslationServer.set_locale(_locale)
	AppState.pending_telemetry.clear()


func test_known_code_uses_its_translation() -> void:
	var vi: Dictionary = PoFile.parse(FileAccess.get_file_as_string("res://locale/vi.po"))
	assert_str(L10n.error_text("TENANT_MISMATCH")).is_equal(String(vi["ui.error.TENANT_MISMATCH"]))
	TranslationServer.set_locale("en-US")
	var en: Dictionary = PoFile.parse(FileAccess.get_file_as_string("res://locale/en.po"))
	assert_str(L10n.error_text("ACCOUNT_LOCKED")).is_equal(String(en["ui.error.ACCOUNT_LOCKED"]))
	assert_int(AppState.pending_telemetry.size()).is_equal(0)


func test_unknown_code_falls_back_to_generic_and_queues_telemetry() -> void:
	var text: String = L10n.error_text("SOMETHING_NEW_FROM_SERVER")
	assert_str(text).is_equal(TranslationServer.translate("ui.error.generic"))
	assert_bool(text.begins_with("ui.")).is_false()
	assert_int(AppState.pending_telemetry.size()).is_equal(1)
	var event: Dictionary = AppState.pending_telemetry[0]
	assert_str(String(event["event"])).is_equal(L10n.TELEMETRY_MISSING_KEY)
	assert_str(String((event["fields"] as Dictionary)["key"])).is_equal("ui.error.SOMETHING_NEW_FROM_SERVER")


func test_every_handled_error_key_is_translated() -> void:
	for key: String in L10n.ERROR_KEYS:
		for locale: String in ["vi-VN", "en-US"]:
			TranslationServer.set_locale(locale)
			assert_bool(L10n.has_key(key)).override_failure_message("%s missing in %s" % [key, locale]).is_true()


func test_pick_follows_locale_then_content_language() -> void:
	var text: Dictionary = {"vi": "Vương quốc Số", "en": "Math Kingdom"}
	assert_str(L10n.pick(text, "en-US")).is_equal("Math Kingdom")
	assert_str(L10n.pick(text, "vi-VN")).is_equal("Vương quốc Số")
	assert_str(L10n.pick({"vi": "Chỉ tiếng Việt"}, "en-US")).is_equal("Chỉ tiếng Việt")
	assert_str(L10n.pick({"en": "English only", "vi": ""}, "vi-VN", "en")).is_equal("English only")
	assert_str(L10n.pick_language({"en": "x"}, "vi-VN")).is_equal("en")
	assert_str(L10n.pick({}, "vi-VN")).is_empty()


## F096-AC-17: 503 UNAVAILABLE and gateway outages without a Problem body read "Hệ thống đang bận…", in vi and en,
## on S01 (LoginFormModel) as everywhere else (L10n.error_text); never the generic message.
func test_service_unavailable_says_the_system_is_busy() -> void:
	for code: String in ["UNAVAILABLE", "HTTP_502", "HTTP_503", "HTTP_504"]:
		assert_str(L10n.error_text(code)).is_equal("Hệ thống đang bận, em thử lại sau ít phút nhé.")
		assert_str(LoginFormModel.message_for(code)).is_equal("Hệ thống đang bận, em thử lại sau ít phút nhé.")
	TranslationServer.set_locale("en-US")
	assert_str(L10n.error_text("HTTP_503")).is_equal("The system is busy. Please try again in a few minutes.")
	assert_int(AppState.pending_telemetry.size()).is_equal(0)
