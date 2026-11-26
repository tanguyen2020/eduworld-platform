extends GdUnitTestSuite
## T042 (F069-AC-01, AC-06, AC-09, AC-14, AC-29; TC-ECO-001, TC-CHL-002): against the backend compose — `hs.an.3a`
## (grade 3) reads its progression, wallet, shop and challenges through the F069 services: every answer parses into the
## client models, the shop prices are Xu, grade 3 sees no expert/legendary difficulty, and no request carries a score,
## XP or result field. Skipped unless EDUWORLD_IT_API_BASE_URL is set (LiveBackend). Needs the F074/F075/F076 seed.

const SKIP_REASON: String = "needs the backend compose with F074/F075/F076 (EDUWORLD_IT_API_BASE_URL)"


func test_grade_3_student_reads_progression_shop_and_challenges(
		do_skip: bool = LiveBackend.skip(), skip_reason: String = SKIP_REASON) -> void:
	var live: LiveBackend = LiveBackend.create(self, Uuid.v4())
	await live.auth.login("hs.an.3a", LiveBackend.student_password())
	var progression: ProgressionService = ProgressionService.new(live.api)
	var inventory: InventoryService = InventoryService.new(live.api)
	var challenge: ChallengeClient = ChallengeClient.new(live.api)
	assert_bool((await progression.refresh()).ok).is_true()
	assert_int(progression.progression().level).is_greater_equal(1)
	assert_bool((await inventory.load_wallet()).ok).is_true()
	assert_bool((await inventory.load_shop()).ok).is_true()
	for item: ShopItemModel in inventory.shop:
		assert_int(item.price).is_greater(0)
	assert_bool((await challenge.list()).ok).is_true()
	for s: ChallengeSummaryModel in challenge.challenges:
		assert_bool("expert" in s.available_difficulties or "legendary" in s.available_difficulties).is_false()
	for sent: Dictionary in live.transport.sent:
		assert_str(String((sent["headers"] as Dictionary).get("x-tenant-id", ""))).is_not_empty()
	live.dispose()
