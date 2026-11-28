extends GdUnitTestSuite
## T018 (F112-AC-16; TC-CHAT-040): the excerpt of a search hit is plain text — markup-looking tags (`[b]`, `[url=…]`,
## `<em>`) are removed and never parsed; the keyword is found without case or Vietnamese accents ("di boss" → "đi boss"),
## each query word at the start of a word; the server's `highlight_ranges` are used when valid, else the client match.


func test_markup_is_removed_and_never_parsed() -> void:
	assert_str(HighlightText.plain("Hẹn [b]đi boss[/b] lúc 19h <em>nhé</em>")).is_equal("Hẹn đi boss lúc 19h nhé")
	assert_str(HighlightText.plain("[url=https://x.invalid]bấm[/url] [color=red]a[/color]")).is_equal("bấm a")
	# A bracket that is not a tag stays as text.
	assert_str(HighlightText.plain("3 [ 4 ] 5")).is_equal("3 [ 4 ] 5")


func test_match_ignores_accents_and_case() -> void:
	var text: String = "Hẹn đi boss lúc 19h nhé"
	var ranges: Array[Vector2i] = HighlightText.match_ranges(text, "di BOSS")
	assert_array(ranges).is_equal([Vector2i(4, 2), Vector2i(7, 4)])
	var pieces: Array[Dictionary] = HighlightText.pieces(text, ranges)
	assert_str(String(pieces[1]["text"])).is_equal("đi")
	assert_bool(bool(pieces[1]["hit"])).is_true()
	assert_str(String(pieces[3]["text"])).is_equal("boss")


func test_query_words_match_word_starts_only() -> void:
	assert_array(HighlightText.match_ranges("Toán học vui", "an")).is_empty()
	assert_array(HighlightText.match_ranges("Toán học vui", "toa")).is_equal([Vector2i(0, 3)])


func test_server_ranges_used_when_valid() -> void:
	var text: String = "Mai mình luyện Toán nhé"
	assert_array(HighlightText.server_ranges(text, [{"start": 15, "length": 4}])).is_equal([Vector2i(15, 4)])
	assert_array(HighlightText.server_ranges(text, [{"start": 40, "length": 4}])).is_empty()
	assert_array(HighlightText.server_ranges(text, "nope")).is_empty()


func test_fold_keeps_positions() -> void:
	var s: String = "Đường đến trường"
	assert_int(HighlightText.fold(s).length()).is_equal(s.length())
	assert_str(HighlightText.fold(s)).is_equal("duong den truong")
