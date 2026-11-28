extends GdUnitTestSuite
## T008 (F111 data-model §1, §3; AGENTS R16, ADR-0095): the F111 models parse their contract bodies with `from_dict`,
## copy no field the contract does not list (never a real name, a school or a grade of another player), drop a legacy
## `study_group` club, pick a tenant name in the UI language with the Vietnamese fallback, count the days left from
## `ends_at` (never hours, minutes or seconds), sort the per-grade contributions and hide them from lớp 1–3, and read the
## bootstrap social policy with the safest default (no "Tạo club") when it is absent.

const NOW: int = 1_763_600_000
const FORBIDDEN: PackedStringArray = ["full_name", "real_name", "school", "school_id", "school_name", "grade", "grade_level",
	"class_id", "email", "score", "mastery", "owner_id"]


func test_suggestion_keeps_only_the_public_card_and_its_context() -> void:
	var raw: Dictionary = F111Fixtures.suggestion(F111Fixtures.SUGGESTED_PARTY, "SoiNho31", "party")
	(raw["player"] as Dictionary)["full_name"] = "Nguyễn Văn A"
	(raw["player"] as Dictionary)["school_name"] = "Trường X"
	raw["grade_level"] = 3
	var s: FriendSuggestionModel = FriendSuggestionModel.from_dict(raw)
	assert_str(s.player.nickname).is_equal("SoiNho31")
	assert_str(s.context_kind).is_equal("party")
	assert_str(s.context_icon()).is_equal("party")
	assert_bool(L10n.has_key(s.context_key())).is_true()
	for obj: Object in [s, s.player]:
		for p: Dictionary in obj.get_property_list():
			assert_bool(String(p["name"]) in FORBIDDEN).override_failure_message(String(p["name"])).is_false()
	# Another source, a bad player id or a nickname longer than 32 → dropped.
	raw["source"] = "classmates"
	assert_object(FriendSuggestionModel.from_dict(raw)).is_null()
	assert_object(FriendSuggestionModel.from_dict({"player": {"user_id": "x", "nickname": "A"}, "source": "recently_played"})).is_null()
	# An unknown context shows no context chip.
	var odd: Dictionary = F111Fixtures.suggestion(F111Fixtures.SUGGESTED_PARTY, "SoiNho31", "lunch")
	assert_str(FriendSuggestionModel.from_dict(odd).context_kind).is_empty()


func test_social_policy_defaults_to_no_club_creation() -> void:
	var none: SocialPolicyModel = SocialPolicyModel.from_policies({})
	assert_str(none.student_club_creation).is_equal("disabled")
	assert_bool(none.can_show_create()).is_false()
	var on: SocialPolicyModel = SocialPolicyModel.from_policies({"social": {"student_club_creation": "auto",
		"club_creation_eligible": true, "cross_school_enabled": true}})
	assert_bool(on.can_show_create()).is_true()
	var not_eligible: SocialPolicyModel = SocialPolicyModel.from_policies({"social": {"student_club_creation": "requires_approval",
		"club_creation_eligible": false}})
	assert_bool(not_eligible.can_show_create()).is_false()
	var odd: SocialPolicyModel = SocialPolicyModel.from_policies({"social": {"student_club_creation": "maybe",
		"club_creation_eligible": true}})
	assert_bool(odd.can_show_create()).is_false()


func test_club_models_drop_study_groups_and_unknown_fields() -> void:
	var list: Array[ClubModel] = ClubModel.list_from(F111Fixtures.mine()["data"])
	assert_int(list.size()).is_equal(4)
	for c: ClubModel in list:
		assert_str(c.club_type).is_not_equal("study_group")
	var own: ClubModel = list[1]
	assert_bool(own.is_officer()).is_true()
	assert_bool(own.is_active()).is_true()
	assert_bool(list[2].is_pending()).is_true()
	assert_bool(list[3].is_read_only()).is_true()
	assert_bool(list[0].is_member()).is_true()
	var discover: ClubModel = ClubModel.from_dict(F111Fixtures.club(F111Fixtures.CLUB_DISCOVER, "CLB Vẽ", "interest_group",
			"active", ""))
	assert_bool(discover.is_member()).is_false()
	assert_bool(L10n.has_key(own.type_key())).override_failure_message(own.type_key()).is_true()
	assert_bool(L10n.has_key(list[2].status_key())).override_failure_message(list[2].status_key()).is_true()
	assert_bool(ResourceLoader.exists(UiBuild.ICONS % own.icon_name())).is_true()
	for p: Dictionary in own.get_property_list():
		assert_bool(String(p["name"]) in FORBIDDEN).override_failure_message(String(p["name"])).is_false()
	var m: ClubMemberModel = ClubMemberModel.from_dict(F111Fixtures.member(F111Fixtures.FRIEND_ONLINE, "CaHeoVui07", "officer"))
	assert_str(m.role).is_equal("officer")
	assert_bool(L10n.has_key(m.role_key())).is_true()
	for p: Dictionary in m.get_property_list():
		assert_bool(String(p["name"]) in FORBIDDEN).override_failure_message(String(p["name"])).is_false()
	var g: ClubGoalModel = ClubGoalModel.list_from(F111Fixtures.goals()["data"])[0]
	assert_float(g.ratio()).is_equal_approx(0.4, 0.001)


func test_event_name_falls_back_to_vietnamese_and_counts_days_only() -> void:
	var events: Array[LiveEventModel] = LiveEventModel.list_from(F111Fixtures.events(NOW)["data"])
	assert_int(events.size()).is_equal(3)
	assert_str(events[0].display_name("en")).is_equal("Mid-Autumn Festival")
	assert_str(events[1].display_name("en")).is_equal("Tuần lễ đọc sách")
	# 5 days less an hour left → 5 days; under a day → the last day; scheduled → "Sắp diễn ra".
	assert_int(events[0].days_left(NOW)).is_equal(5)
	assert_str(events[0].time_left_key(NOW)).is_equal("ui.events.time_left_days")
	assert_str(events[1].time_left_key(NOW)).is_equal("ui.events.last_day")
	assert_str(events[2].time_left_key(NOW)).is_equal("ui.events.upcoming")
	var text: String = events[0].time_left_text(NOW)
	for unit: String in ["giờ", "phút", "giây", ":"]:
		assert_bool(text.contains(unit)).override_failure_message(text).is_false()


func test_community_goal_sorts_grades_and_hides_them_from_g1_3() -> void:
	var g: CommunityGoalModel = CommunityGoalModel.from_dict(F111Fixtures.goal(6200, [{"grade_level": 7, "progress": 300},
		{"grade_level": 4, "progress": 900}, {"grade_level": 5, "progress": 120}]))
	assert_float(g.ratio()).is_equal_approx(0.62, 0.001)
	var rows: Array[Dictionary] = g.grade_rows("g6_7")
	assert_array(rows.map(func(r: Dictionary) -> int: return int(r["grade_level"]))).is_equal([4, 5, 7])
	assert_array(g.grade_rows("g1_3")).is_empty()
	# A grade the server hid (< 5 students) is not invented.
	assert_bool(rows.any(func(r: Dictionary) -> bool: return int(r["grade_level"]) == 6)).is_false()
	for p: Dictionary in g.get_property_list():
		assert_bool(String(p["name"]) in ["rank", "ranking", "top", "names"]).is_false()


func test_season_track_reads_every_tier() -> void:
	var t: SeasonTrackModel = SeasonTrackModel.from_dict(F111Fixtures.season(30))
	assert_int(t.tiers.size()).is_equal(30)
	assert_int(t.tier).is_equal(3)
	assert_bool(t.tier_model(2).reached).is_true()
	assert_bool(t.tier_model(2).claimed).is_false()
	assert_bool(t.tier_model(10).milestone).is_true()
	assert_str(t.display_name("en")).is_equal("Discovery Season")
	var h: Array[SeasonHistoryItemModel] = SeasonHistoryItemModel.list_from(F111Fixtures.history()["data"])
	assert_int(h[0].final_tier).is_equal(12)
