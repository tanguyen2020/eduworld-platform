extends GdUnitTestSuite
## F060-AC-16 (NFR-I18N-003, TC-I18N-007): numbers, dates, times and durations follow GODOT_ARCHITECTURE §10.1
## for vi-VN and en-US, whatever the current or OS locale.

const MOMENT: Dictionary = {"year": 2026, "month": 11, "day": 15, "hour": 14, "minute": 30, "second": 0}

var _locale: String = ""


func before_test() -> void:
	_locale = TranslationServer.get_locale()


func after_test() -> void:
	TranslationServer.set_locale(_locale)


func test_numbers() -> void:
	assert_str(LocaleFormat.number(1234.5, "vi-VN")).is_equal("1.234,5")
	assert_str(LocaleFormat.number(1234.5, "en-US")).is_equal("1,234.5")
	assert_str(LocaleFormat.number(1234567, "vi-VN")).is_equal("1.234.567")
	assert_str(LocaleFormat.number(-0.25, "en-US", 2)).is_equal("-0.25")
	assert_str(LocaleFormat.number(12, "en-US")).is_equal("12")


func test_dates() -> void:
	assert_str(LocaleFormat.date(MOMENT, "vi-VN")).is_equal("15/11/2026")
	assert_str(LocaleFormat.date(MOMENT, "en-US")).is_equal("Nov 15, 2026")


func test_times() -> void:
	assert_str(LocaleFormat.time(MOMENT, "vi-VN")).is_equal("14:30")
	assert_str(LocaleFormat.time(MOMENT, "en-US")).is_equal("2:30 PM")
	assert_str(LocaleFormat.time({"hour": 0, "minute": 5}, "en-US")).is_equal("12:05 AM")


func test_durations() -> void:
	assert_str(LocaleFormat.duration(200, "vi-VN")).is_equal("3 phút 20 giây")
	assert_str(LocaleFormat.duration(200, "en-US")).is_equal("3 min 20 s")
	assert_str(LocaleFormat.duration(45, "vi-VN")).is_equal("45 giây")
	assert_str(LocaleFormat.duration(3725, "en-US")).is_equal("1 h 2 min 5 s")


func test_independent_of_current_locale() -> void:
	TranslationServer.set_locale("en-US")
	assert_str(LocaleFormat.number(1234.5, "vi-VN")).is_equal("1.234,5")
	TranslationServer.set_locale("vi-VN")
	assert_str(LocaleFormat.date(MOMENT, "en-US")).is_equal("Nov 15, 2026")
	assert_str(LocaleFormat.number(1234.5)).is_equal("1.234,5")
