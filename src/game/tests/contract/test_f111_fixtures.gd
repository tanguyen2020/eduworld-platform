extends GdUnitTestSuite
## T007 (F111 contracts §6, GODOT_ARCHITECTURE §12): every F111 fixture generated from the contract
## (tools/gen_fixtures.mjs: OpenAPI `FriendSuggestion`, `Club*`, `GameEvent*`, `EventMilestone`, `CommunityGoalProgress`,
## `RewardClaimResult`, `SeasonTrack*`, `QuestSummary`, the `listFriendSuggestions` / `listMyClubInvitations` /
## `listClubGoals` examples, the bootstrap `policies.social`; AsyncAPI `event.*`, `season.track.updated`, the club
## notifications) parses into the typed client models, and every realtime sample passes the payload checks of the
## dispatcher (WsMessageSchemas).

const F111_MESSAGES: PackedStringArray = ["event.status.changed", "event.progress.updated", "event.community_goal.updated",
	"season.track.updated"]


func test_social_fixtures_parse() -> void:
	var list: Array[FriendSuggestionModel] = FriendSuggestionModel.list_from(
			F111Fixtures.openapi("listFriendSuggestions.200")["data"])
	assert_int(list.size()).is_equal(2)
	assert_str(list[0].player.nickname).is_equal("GauTruc22")
	assert_str(list[0].context_kind).is_equal("coop")
	assert_object(FriendSuggestionModel.from_dict(F111Fixtures.openapi("FriendSuggestion"))).is_not_null()
	var policy: SocialPolicyModel = SocialPolicyModel.from_policies(
			(F111Fixtures.openapi("GameLoginResponse")["bootstrap"] as Dictionary)["policies"] as Dictionary)
	assert_str(policy.student_club_creation).is_equal("requires_approval")
	assert_bool(policy.club_creation_eligible).is_true()
	assert_bool(policy.cross_school_enabled).is_false()


func test_club_fixtures_parse() -> void:
	var c: Dictionary = F111Fixtures.openapi("Club")
	c["club_type"] = "interest_group"
	assert_object(ClubModel.from_dict(c)).is_not_null()
	assert_object(ClubMemberModel.from_dict(F111Fixtures.openapi("ClubMember"))).is_not_null()
	assert_object(ClubGoalModel.from_dict(F111Fixtures.openapi("ClubGoal"))).is_not_null()
	assert_object(ClubEventModel.from_dict(F111Fixtures.openapi("ClubEvent"))).is_not_null()
	assert_object(ClubJoinRequestModel.from_dict(F111Fixtures.openapi("ClubJoinRequest"))).is_not_null()
	assert_object(ClubInvitationModel.from_dict(F111Fixtures.openapi("ClubInvitation"))).is_not_null()
	var invitations: Array[ClubInvitationModel] = ClubInvitationModel.list_from(
			F111Fixtures.openapi("listMyClubInvitations.200")["data"])
	assert_int(invitations.size()).is_equal(1)
	assert_str(invitations[0].club_name).is_equal("Nhà thám hiểm nhí")
	assert_str(invitations[0].inviter_nickname).is_equal("CaHeoVui07")
	var goals: Array[ClubGoalModel] = ClubGoalModel.list_from(F111Fixtures.openapi("listClubGoals.200")["data"])
	assert_int(goals.size()).is_equal(1)
	assert_int(goals[0].my_contribution).is_equal(3)


func test_liveops_fixtures_parse() -> void:
	assert_object(LiveEventModel.from_dict(F111Fixtures.openapi("GameEventSummary"))).is_not_null()
	var detail: LiveEventModel = LiveEventModel.from_dict(F111Fixtures.openapi("GameEventDetail"))
	assert_object(detail).is_not_null()
	assert_int(detail.milestones.size()).is_equal(1)
	assert_object(detail.community_goal).is_not_null()
	assert_object(EventMilestoneModel.from_dict(F111Fixtures.openapi("EventMilestone"))).is_not_null()
	assert_object(CommunityGoalModel.from_dict(F111Fixtures.openapi("CommunityGoalProgress"))).is_not_null()
	var claim: Dictionary = F111Fixtures.openapi("RewardClaimResult")
	assert_int(RewardModel.list_from(claim["rewards"]).size()).is_equal(1)
	var track: SeasonTrackModel = SeasonTrackModel.from_dict(F111Fixtures.openapi("SeasonTrack"))
	assert_object(track).is_not_null()
	assert_int(track.tiers.size()).is_equal(1)
	assert_object(SeasonTierModel.from_dict(F111Fixtures.openapi("SeasonTrackTier"))).is_not_null()
	assert_object(SeasonHistoryItemModel.from_dict(F111Fixtures.openapi("SeasonTrackHistoryItem"))).is_not_null()
	var q: QuestModel = QuestModel.from_dict(F111Fixtures.openapi("QuestSummary"))
	assert_str(q.available_until).is_equal("2026-11-15T07:30:00Z")


func test_realtime_samples_pass_the_dispatcher_checks() -> void:
	for type: String in F111_MESSAGES:
		var env: Dictionary = F111Fixtures.asyncapi("%s.sample" % type)
		assert_bool(WsMessageSchemas.is_known(type)).override_failure_message(type).is_true()
		assert_bool(WsMessageSchemas.validate(type, env["payload"] as Dictionary)).override_failure_message(type).is_true()
	# A malformed payload is dropped before a service sees it.
	assert_bool(WsMessageSchemas.validate("event.status.changed", {"event_id": "x"})).is_false()
	assert_bool(WsMessageSchemas.validate("season.track.updated", {"season_id": "x", "points": "ten", "tier": 1})).is_false()


func test_club_notifications_open_the_club_and_the_event_center() -> void:
	for kind: String in ["club_invitation", "club_join_request", "club_approved"]:
		var payload: Dictionary = F111Fixtures.asyncapi("notification.item.created.%s" % kind)["payload"] as Dictionary
		var item: NotificationModel = NotificationModel.from_dict(payload)
		assert_object(item).override_failure_message(kind).is_not_null()
		assert_str(item.action_kind).is_equal("open_club")
		assert_str(String(NotificationService.new().target_screen(item))).is_equal("friends")
