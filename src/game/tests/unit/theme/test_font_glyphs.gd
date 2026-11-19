extends GdUnitTestSuite
## F060-AC-18 (NFR-I18N-003, NFR-A11Y-001): every font of the generated Theme covers Vietnamese with diacritics and
## extended Latin — no missing glyph for the sample set.

const SAMPLE_LINES: PackedStringArray = [
	"Tiếng Việt: ằẳẵặ ỹỷỵ ữựừ Đđ Ơơ Ưư ăâêô",
	"àảãáạ ầẩẫấậ èẻẽéẹ ềểễếệ ìỉĩíị òỏõóọ ồổỗốộ ờởỡớợ ùủũúụ ỳỷỹýỵ",
	"ÀẢÃÁẠ ĂẰẲẴẮẶ ÂẦẨẪẤẬ ÈẺẼÉẸ ÊỀỂỄẾỆ ÌỈĨÍỊ ÒỎÕÓỌ ÔỒỔỖỐỘ",
	"ƠỜỞỠỚỢ ÙỦŨÚỤ ƯỪỬỮỨỰ ỲỶỸÝỴ äöüßçñ 0123456789 %–…",
]


func _theme_fonts() -> Array[Font]:
	var fonts: Array[Font] = []
	for band: String in UiTokens.DENSITY_BANDS:
		var theme: Theme = load(String(UiTokens.THEME_PATHS[band])) as Theme
		if theme.default_font != null and not theme.default_font in fonts:
			fonts.append(theme.default_font)
		for type_name: String in theme.get_type_list():
			for font_name: String in theme.get_font_list(type_name):
				var font: Font = theme.get_font(font_name, type_name)
				if font != null and not font in fonts:
					fonts.append(font)
	return fonts


func test_theme_has_body_bold_and_heading_fonts() -> void:
	assert_int(_theme_fonts().size()).is_greater_equal(3)


func test_every_theme_font_covers_vietnamese() -> void:
	for font: Font in _theme_fonts():
		var missing: Array[String] = []
		for c: String in "".join(SAMPLE_LINES):
			if c != " " and not font.has_char(c.unicode_at(0)):
				missing.append(c)
		assert_array(missing).override_failure_message("%s lacks %s" % [font.resource_path, missing]).is_empty()
