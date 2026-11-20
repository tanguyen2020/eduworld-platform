extends GdUnitTestSuite
## F070-AC-18 (T028, CHARACTER_SYSTEM §5.2): `avatar_builder.gd` draws an AvatarConfig from the display resources
## (`resources/items`, `resources/avatar`) — skin tone and hair colour from the avatar palette tokens (T041), items on their slot anchors — and,
## when an equipped item has no visual on this client (cosmetic bundle not loaded), shows the default item of the same
## slot without crashing. No network.

const RIG: String = "res://scenes/character/AvatarRig.tscn"

var catalog: CharacterCatalogModel = null


func before_test() -> void:
	catalog = ProfileFixtures.catalog_model()


func _rig() -> AvatarBuilder:
	var rig: AvatarBuilder = auto_free((load(RIG) as PackedScene).instantiate()) as AvatarBuilder
	add_child(rig)
	return rig


func _config(overrides: Dictionary = {}) -> AvatarConfigModel:
	var d: Dictionary = (ProfileFixtures.profile_ready()["avatar_config"] as Dictionary).duplicate(true)
	d.merge(overrides, true)
	return AvatarConfigModel.from_dict(d)


func test_every_slot_is_drawn_from_its_visual() -> void:
	var rig: AvatarBuilder = _rig()
	rig.apply(_config(), catalog)
	assert_str(String(rig.shown["hair"])).is_equal("hair_bob_01")
	assert_str(String(rig.shown["outfit"])).is_equal("outfit_casual_01")
	assert_str(String(rig.shown["face"])).is_equal("acc_glasses_round")
	assert_bool(rig.shown.has("head")).is_false()
	assert_array(Array(rig.missing)).is_empty()
	assert_int(rig.slot_anchor("hair").get_child_count()).is_equal(1)
	assert_int(rig.slot_anchor("head").get_child_count()).is_equal(0)
	assert_object(rig.skin_color()).is_equal(Art3dTokens.AVATAR_SKIN_TONES["tone_04"] as Color)
	assert_object(rig.hair_color()).is_equal(Art3dTokens.AVATAR_HAIR_COLORS["hair_col_03"] as Color)


func test_missing_item_visual_shows_the_default_of_the_same_slot() -> void:
	var rig: AvatarBuilder = _rig()
	var fallbacks: Array[Array] = []
	rig.visual_fallback_used.connect(func(item_id: String, slot: String) -> void: fallbacks.append([item_id, slot]))
	rig.apply(_config({"outfit": {"item_id": "outfit_galaxy_99"},
			"accessories": [{"slot": "head", "item_id": "acc_crown_rare_77"}]}), catalog)
	assert_str(String(rig.shown["outfit"])).is_equal(ItemVisual.first_for_slot("outfit", catalog.default_item_ids))
	assert_str(String(rig.shown["outfit"])).is_equal("outfit_explorer_01")
	assert_str(String(rig.shown["head"])).is_equal("acc_cap_star")
	assert_array(Array(rig.missing)).contains_exactly_in_any_order(["outfit_galaxy_99", "acc_crown_rare_77"])
	assert_array(fallbacks).contains([["outfit_galaxy_99", "outfit"], ["acc_crown_rare_77", "head"]])
	assert_int(rig.slot_anchor("outfit").get_child_count()).is_equal(1)
	# The equipped config is kept as the server sent it (the fallback is display only).
	assert_str(rig.config.outfit_item_id).is_equal("outfit_galaxy_99")


func test_missing_visual_without_catalog_or_default_does_not_crash() -> void:
	var rig: AvatarBuilder = _rig()
	rig.apply(_config({"hair": {"item_id": "hair_unknown_42", "color": "hair_col_99"}, "skin_tone": "tone_99",
			"body_type": "kid_zz"}), null)
	assert_bool(rig.shown.has("hair")).is_false()
	assert_int(rig.slot_anchor("hair").get_child_count()).is_equal(0)
	assert_array(Array(rig.missing)).contains(["hair_unknown_42"])
	rig.apply(null, null)
	assert_int(rig.slot_anchor("outfit").get_child_count()).is_equal(0)


func test_codes_are_never_used_as_paths() -> void:
	var rig: AvatarBuilder = _rig()
	rig.apply(_config({"outfit": {"item_id": "../../project"}}), catalog)
	assert_str(String(rig.shown["outfit"])).is_equal("outfit_explorer_01")
	assert_object(ItemVisual.load_item("../../project")).is_null()


func test_reapplying_replaces_the_previous_parts() -> void:
	var rig: AvatarBuilder = _rig()
	rig.apply(_config(), catalog)
	rig.apply(_config({"hair": {"item_id": "hair_spiky_01", "color": "hair_col_08"}, "accessories": []}), catalog)
	await get_tree().process_frame
	assert_int(rig.slot_anchor("hair").get_child_count()).is_equal(1)
	assert_int(rig.slot_anchor("face").get_child_count()).is_equal(0)
	assert_str(String(rig.shown["hair"])).is_equal("hair_spiky_01")
	assert_object(rig.hair_color()).is_equal(ItemVisual.appearance_color("hair_col_08"))


func test_every_seed_code_has_a_visual() -> void:
	for item_id: String in catalog.default_item_ids:
		assert_object(ItemVisual.load_item(item_id)).override_failure_message(item_id).is_not_null()
	for emote_id: String in catalog.default_emote_ids:
		var visual: ItemVisual = ItemVisual.load_item(emote_id)
		assert_object(visual).override_failure_message(emote_id).is_not_null()
		assert_str(visual.slot).is_equal(ItemVisual.SLOT_EMOTE)
	for code: String in CharacterCatalogModel.codes_of(catalog.body_types):
		assert_object(ItemVisual.load_appearance(code)).override_failure_message(code).is_not_null()
	# T041: every skin tone and hair colour of the catalog is a palette token key (no colour resource on the client).
	for list: Array[Dictionary] in [catalog.skin_tones, catalog.hair_colors]:
		for code: String in CharacterCatalogModel.codes_of(list):
			assert_bool(ItemVisual.has_appearance_color(code)).override_failure_message(code).is_true()
			assert_bool(ResourceLoader.exists("%s/%s.tres" % [ItemVisual.APPEARANCE_DIR, code])).is_false()


## T041: a code without a token (catalog newer than the client) draws the neutral colour instead of failing.
func test_unknown_palette_code_uses_the_neutral_colour() -> void:
	assert_object(ItemVisual.appearance_color("tone_99")).is_equal(ItemVisual.UNKNOWN_APPEARANCE_COLOR)
	assert_bool(ItemVisual.has_appearance_color("tone_99")).is_false()
	var rig: AvatarBuilder = _rig()
	rig.apply(_config({"skin_tone": "tone_99"}), catalog)
	assert_object(rig.skin_color()).is_equal(ItemVisual.UNKNOWN_APPEARANCE_COLOR)
