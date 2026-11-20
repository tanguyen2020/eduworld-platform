extends GdUnitTestSuite
## F065-AC-16 (TC-VID-004; T025): WebVTT parsed by the client — timing with or without hours, cue identifiers, multi-line
## text without tags, overlapping cues joined; the subtitle language follows the UI language, else the default track.


func test_parses_cues_and_text_at_a_position() -> void:
	var p: VttParser = VttParser.new()
	assert_bool(p.parse(VideoFixtures.VTT_VI)).is_true()
	assert_int(p.cues.size()).is_equal(2)
	assert_int(p.cues[1].start_ms).is_equal(5000)
	assert_int(p.cues[1].end_ms).is_equal(8500)
	assert_str(p.text_at(1000)).is_equal("Xin chào các em!")
	assert_str(p.text_at(4500)).is_empty()
	assert_str(p.text_at(6000)).is_equal("Hôm nay mình học\nphép nhân.")


func test_hours_bom_and_overlaps() -> void:
	var p: VttParser = VttParser.new()
	assert_bool(p.parse("﻿WEBVTT - bài 1\r\n\r\nNOTE ghi chú\r\n\r\n01:00:00.000 --> 01:00:02.000 line:90%\r\nA\r\n\r\n01:00:01.000 --> 01:00:03.000\r\nB\r\n")).is_true()
	assert_int(p.cues[0].start_ms).is_equal(3_600_000)
	assert_str(p.text_at(3_601_500)).is_equal("A\nB")


func test_rejects_non_vtt_and_skips_broken_cues() -> void:
	var p: VttParser = VttParser.new()
	assert_bool(p.parse("1\n00:00:01,000 --> 00:00:02,000\nSRT")).is_false()
	assert_bool(p.parse("WEBVTT\n\n00:05.000 --> 00:01.000\nbackwards\n\nxx --> yy\nbad")).is_true()
	assert_int(p.cues.size()).is_equal(0)
	assert_int(VttParser.parse_time("02:03.5")).is_equal(123500)
	assert_int(VttParser.parse_time("a:b.c")).is_equal(-1)


func test_language_follows_the_ui_then_the_default() -> void:
	var tracks: Array[Dictionary] = [{"language": "vi", "is_default": true}, {"language": "en", "is_default": false}]
	assert_str(VttParser.pick_language(tracks, "en_US")).is_equal("en")
	assert_str(VttParser.pick_language(tracks, "vi")).is_equal("vi")
	assert_str(VttParser.pick_language(tracks, "fr")).is_equal("vi")
	var none: Array[Dictionary] = []
	assert_str(VttParser.pick_language(none, "vi")).is_empty()
