extends GdUnitTestSuite
## F070 T040 (OQ F070-1): the character catalog carries a name vi/en per default item and emote (`items`); the client
## shows it in the student's language, marks a name that only exists in another language with
## `ui.learning.no_translation`, and keeps the ordinal label when the catalog has no entry (version published before
## the names). The emote wheel and the creation carousels use these names.

const WHEEL: String = "res://scenes/character/EmoteWheel.tscn"

var _locale: String = ""


func before_test() -> void:
	_locale = TranslationServer.get_locale()


func after_test() -> void:
	TranslationServer.set_locale(_locale)


func test_items_are_parsed_with_slot_and_names() -> void:
	var catalog: CharacterCatalogModel = ProfileFixtures.catalog_model()
	assert_int(catalog.items.size()).is_equal(catalog.default_item_ids.size() + catalog.default_emote_ids.size())
	var hair: Dictionary = catalog.items["hair_short_01"] as Dictionary
	assert_str(String(hair["equip_slot"])).is_equal("hair")
	assert_str(String((hair["name"] as Dictionary)["vi"])).is_equal("Tóc ngắn")
	assert_str(String((catalog.items["emote_wave"] as Dictionary)["equip_slot"])).is_equal("emote")


func test_name_follows_the_locale() -> void:
	var catalog: CharacterCatalogModel = ProfileFixtures.catalog_model()
	assert_str(catalog.item_label("hair_short_01", "x", "vi-VN")).is_equal("Tóc ngắn")
	assert_str(catalog.item_label("hair_short_01", "x", "en-US")).is_equal("Short hair")
	var n: Dictionary = catalog.item_name("emote_wave", "en-US")
	assert_str(String(n["text"])).is_equal("Wave")
	assert_bool(bool(n["translation_missing"])).is_false()


func test_missing_translation_shows_the_other_language_with_the_no_translation_label() -> void:
	var data: Dictionary = ProfileFixtures.catalog()
	for entry: Variant in data["items"] as Array:
		if String((entry as Dictionary)["item_id"]) == "outfit_sport_01":
			((entry as Dictionary)["name"] as Dictionary).erase("en")
	var catalog: CharacterCatalogModel = CharacterCatalogModel.from_dict(data)
	TranslationServer.set_locale("en")
	var n: Dictionary = catalog.item_name("outfit_sport_01", "en-US")
	assert_str(String(n["text"])).is_equal("Bộ thể thao")
	assert_bool(bool(n["translation_missing"])).is_true()
	var label: String = catalog.item_label("outfit_sport_01", "x", "en-US")
	assert_str(label).is_equal("Bộ thể thao (%s)" % tr(CharacterCatalogModel.NO_TRANSLATION_KEY))
	assert_str(tr(CharacterCatalogModel.NO_TRANSLATION_KEY)).is_not_equal(CharacterCatalogModel.NO_TRANSLATION_KEY)


func test_unknown_item_or_catalog_without_items_keeps_the_ordinal_label() -> void:
	var catalog: CharacterCatalogModel = ProfileFixtures.catalog_model()
	assert_str(catalog.item_label("hair_unknown_99", "Mẫu 3")).is_equal("Mẫu 3")
	var data: Dictionary = ProfileFixtures.catalog()
	data.erase("items")
	var old: CharacterCatalogModel = CharacterCatalogModel.from_dict(data)
	assert_object(old).is_not_null()
	assert_bool(old.items.is_empty()).is_true()
	assert_str(old.item_label("hair_short_01", "Mẫu 1")).is_equal("Mẫu 1")


func test_emote_wheel_shows_the_emote_names() -> void:
	TranslationServer.set_locale("vi")
	var wheel: EmoteWheel = auto_free((load(WHEEL) as PackedScene).instantiate()) as EmoteWheel
	add_child(wheel)
	await get_tree().process_frame
	wheel.catalog = ProfileFixtures.catalog_model()
	wheel.set_emotes(PackedStringArray(["emote_wave", "emote_clap"]))
	var texts: Array = wheel.slot_buttons().map(func(b: Button) -> String: return b.text)
	assert_array(texts).contains_exactly(["Vẫy tay", "Vỗ tay"])
