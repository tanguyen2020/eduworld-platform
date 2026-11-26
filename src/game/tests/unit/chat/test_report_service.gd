extends GdUnitTestSuite
## T018 (F119-AC-13, AC-14; TC-CHAT-006, MODERATION §5): ReportService sends exactly `ReportCreate` — target, a reason of
## the enum, the match / party / channel context — with one `Idempotency-Key` per report, kept when the same report is
## retried after a network failure and new for another report; nothing of the chat text or a nickname is sent.

var s: ChatSceneHarness = null


func before_test() -> void:
	s = ChatSceneHarness.new()
	await s.start()


func after_test() -> void:
	s.dispose()


func test_report_of_a_message_sends_only_the_contract_fields() -> void:
	var r: ApiResult = await s.services.reports.create_report("message", ChatFixtures.MSG_MASKED, "rude_language",
			{"channel_id": ChatFixtures.PARTY_CHANNEL, "party_id": ChatFixtures.PARTY_ID, "nickname": "CaHeoVui07"})
	assert_bool(r.ok).is_true()
	var rec: FakeHttp.Recorded = s.h.http.last(ChatFixtures.ROUTE_REPORT)
	assert_dict(rec.body).is_equal({"target_type": "message", "target_id": ChatFixtures.MSG_MASKED, "reason": "rude_language",
		"context": {"channel_id": ChatFixtures.PARTY_CHANNEL, "party_id": ChatFixtures.PARTY_ID}})
	assert_bool(Uuid.is_valid(String(rec.headers.get("idempotency-key", "")))).is_true()
	assert_str(rec.raw_body).not_contains("CaHeoVui07")


func test_report_of_a_player_from_a_match_carries_the_match_id() -> void:
	await s.services.reports.create_report("user", ChatFixtures.OTHER, "bullying", {"match_id": ChatFixtures.MATCH_ID})
	var rec: FakeHttp.Recorded = s.h.http.last(ChatFixtures.ROUTE_REPORT)
	assert_str(String(rec.body["target_type"])).is_equal("user")
	assert_dict(rec.body["context"] as Dictionary).is_equal({"match_id": ChatFixtures.MATCH_ID})


func test_retry_after_network_failure_keeps_the_key_and_a_new_report_gets_a_new_one() -> void:
	s.h.http.clear(ChatFixtures.ROUTE_REPORT).fail(ChatFixtures.ROUTE_REPORT)
	var first: ApiResult = await s.services.reports.create_report("message", ChatFixtures.MSG_TEXT, "spam")
	assert_bool(first.ok).is_false()
	var keys: Array[String] = []
	for rec: FakeHttp.Recorded in s.h.http.find(ChatFixtures.ROUTE_REPORT):
		keys.append(String(rec.headers.get("idempotency-key", "")))
	s.h.http.clear(ChatFixtures.ROUTE_REPORT).reply(ChatFixtures.ROUTE_REPORT, 201, ChatFixtures.report_created())
	var again: ApiResult = await s.services.reports.create_report("message", ChatFixtures.MSG_TEXT, "spam")
	assert_bool(again.ok).is_true()
	var retry_key: String = String(s.h.http.last(ChatFixtures.ROUTE_REPORT).headers.get("idempotency-key", ""))
	assert_str(retry_key).is_equal(keys[0])
	await s.services.reports.create_report("message", ChatFixtures.MSG_TEXT, "bullying")
	assert_str(String(s.h.http.last(ChatFixtures.ROUTE_REPORT).headers.get("idempotency-key", ""))).is_not_equal(retry_key)


func test_reason_outside_the_enum_is_never_sent() -> void:
	var r: ApiResult = await s.services.reports.create_report("message", ChatFixtures.MSG_TEXT, "made_up")
	assert_bool(r.ok).is_false()
	assert_int(s.h.http.count(ChatFixtures.ROUTE_REPORT)).is_equal(0)
	assert_array(Array(ReportService.REASONS)).is_equal(["rude_language", "bullying", "asked_personal_info", "spam",
		"inappropriate_content", "cheating", "other"])
