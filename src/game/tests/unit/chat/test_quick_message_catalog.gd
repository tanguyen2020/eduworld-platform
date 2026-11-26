extends GdUnitTestSuite
## T013 catalog side (F119-AC-08; TC-I18N-008, AC-UX-107, ADR-0056): quick messages come from `listQuickMessages`, are
## ordered by `sort_order`, and each viewer reads them in their own language — a Vietnamese sender's "Chúc may mắn!"
## reads "Good luck!" for an English viewer; an id the catalog does not know shows a neutral placeholder.

var catalog: QuickMessageCatalog = null


func before_test() -> void:
	catalog = QuickMessageCatalog.new()
	catalog.load_list(ChatFixtures.quick()["data"], "party")


func test_catalog_is_loaded_in_sort_order() -> void:
	assert_bool(catalog.is_loaded()).is_true()
	assert_str(catalog.items[0].id).is_equal("qm_hello")
	assert_str(catalog.items[catalog.items.size() - 1].id).is_equal("qm_good_luck")


func test_each_viewer_reads_the_sentence_in_their_own_language() -> void:
	assert_str(catalog.text_for("qm_good_luck", {}, "vi-VN")).is_equal("Chúc may mắn!")
	assert_str(catalog.text_for("qm_good_luck", {}, "en-US")).is_equal("Good luck!")


func test_message_follows_the_viewer_locale_when_it_changes() -> void:
	var m: ChatMessageModel = ChatMessageModel.from_dict(ChatFixtures.ws("chat.message.created.quick"))
	var before: String = TranslationServer.get_locale()
	TranslationServer.set_locale("en_US")
	var en: String = ChatText.visible_text(m, catalog)
	TranslationServer.set_locale("vi_VN")
	var vi: String = ChatText.visible_text(m, catalog)
	TranslationServer.set_locale(before)
	assert_str(en).is_equal("Good luck!")
	assert_str(vi).is_equal("Chúc may mắn!")


func test_parameters_are_inserted_and_unknown_ids_are_neutral() -> void:
	catalog.load_list([{"quick_message_id": "qm_n", "category": "teamwork", "text": {"vi": "Còn {n} câu!", "en": "{n} left!"}}])
	assert_str(catalog.text_for("qm_n", {"n": "2"}, "en")).is_equal("2 left!")
	assert_str(catalog.text_for("qm_missing")).is_equal(tr(QuickMessageCatalog.KEY_UNKNOWN))
