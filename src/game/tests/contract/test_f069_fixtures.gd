extends GdUnitTestSuite
## F069 contract fixtures (T006; GODOT_ARCHITECTURE §12): every F069 example generated from specs/openapi by
## tools/gen_fixtures.mjs parses into its client model (unknown fields ignored, required ones present).

func _f(schema: String) -> Dictionary:
	return AuthHarness.fixture(schema)


func test_progression_fixtures_parse() -> void:
	assert_object(ProgressionModel.from_dict(_f("ProgressionStatus"))).is_not_null()
	assert_object(XpEntryModel.from_dict(_f("XpLedgerEntry"))).is_not_null()
	assert_object(LevelModel.from_dict(_f("ProgressionLevel"))).is_not_null()
	var p: ProgressionModel = ProgressionModel.from_dict(_f("ProgressionStatus"))
	assert_int(p.next_level_rewards.size()).is_equal(1)


func test_economy_fixtures_parse() -> void:
	assert_object(WalletModel.from_dict(_f("Wallet"))).is_not_null()
	assert_object(LedgerEntryModel.from_dict(_f("CurrencyLedgerEntry"), LedgerEntryModel.LEDGER_CURRENCY)).is_not_null()
	assert_object(LedgerEntryModel.from_dict(_f("ItemLedgerEntry"), LedgerEntryModel.LEDGER_ITEM)).is_not_null()
	assert_object(InventoryItemModel.from_dict(_f("InventoryItem"))).is_not_null()
	assert_object(ShopItemModel.from_dict(_f("ShopItem"))).is_not_null()
	var purchase: Dictionary = _f("PurchaseResult")
	assert_bool(purchase.has("balance_after")).is_true()
	assert_object(InventoryItemModel.from_dict(purchase["item"])).is_not_null()


func test_achievement_and_collection_fixtures_parse() -> void:
	assert_object(AchievementModel.from_dict(_f("AchievementProgress"))).is_not_null()
	assert_object(CollectionModel.from_dict(_f("CollectionProgress"))).is_not_null()
	var detail: CollectionModel = CollectionModel.from_dict(_f("CollectionDetail"))
	assert_int(detail.items.size()).is_equal(1)
	assert_str(L10n.pick(detail.items[0]["how_to_obtain"] as Dictionary)).is_not_empty()


func test_challenge_fixtures_parse() -> void:
	assert_object(ChallengeSummaryModel.from_dict(_f("GameChallengeSummary"))).is_not_null()
	var detail: ChallengeDetailModel = ChallengeDetailModel.detail_from(_f("GameChallengeDetail"))
	assert_object(detail).is_not_null()
	assert_array(detail.difficulty_options()).is_not_empty()
	assert_object(ChallengeAttemptModel.from_dict(_f("GameChallengeAttempt"))).is_not_null()
	var next: Dictionary = _f("ChallengeNextActivity")
	assert_object(ChallengeAttemptModel.from_dict(next["challenge_attempt"])).is_not_null()
	assert_bool(next.has("done")).is_true()
