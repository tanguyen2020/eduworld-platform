extends GdUnitTestSuite
## T008 (F112-AC-01, AC-03, AC-04, AC-05, AC-10; TC-CHAT-002, TC-CHAT-008, TC-CHAT-039): the tabs of the HUD chat come
## only from the channels the server listed — "Khu vực" (only in a hub instance), "Cả trường", one per club of the student
## whose channel is listed, "Bạn bè", the DMs opened — no tab for an unknown type or per class / grade; a new `area`
## channel replaces the old one (leave + drop); World / Area never count unread (only "@"); the HUD badge is the sum of the
## club / DM unread counts; a DM whose friend is unknown only adds to the "Bạn bè" badge, never a guessed name.

var state: ChatHubState = null
var store: ChatStore = null


func before_test() -> void:
	state = ChatHubState.new()
	store = ChatStore.new()


func _listed(body: Dictionary) -> Array[ChatChannelModel]:
	var out: Array[ChatChannelModel] = []
	for raw: Variant in body["data"] as Array:
		var c: ChatChannelModel = ChatChannelModel.from_dict(raw)
		if c != null:
			out.append(c)
			store.ensure_channel(c)
	return out


func _clubs() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for raw: Variant in F112Fixtures.mine()["data"] as Array:
		var c: ClubModel = ClubModel.from_dict(raw)
		if c != null and c.is_member() and not c.chat_channel_id.is_empty():
			out.append({"club_id": c.club_id, "name": c.name, "chat_channel_id": c.chat_channel_id})
	return out


func _friends() -> Array[Dictionary]:
	var out: Array[Dictionary] = []
	for raw: Variant in F112Fixtures.openapi("listFriends.200")["data"] as Array:
		var d: Dictionary = raw as Dictionary
		out.append({"user_id": d["user_id"], "nickname": d["nickname"], "level": d["level"]})
	return out


func test_tabs_come_only_from_listed_channels_in_order() -> void:
	var extra: Array = [{"channel_id": "0193b0a0-0000-7000-8000-0000000005c1", "channel_type": "class",
		"allowed_kinds": ["text"]}, {"channel_id": "0193b0a0-0000-7000-8000-0000000005c2", "channel_type": "study_group",
		"allowed_kinds": ["text"]}]
	state.rebuild(_listed(F112Fixtures.channels(extra)), store, _clubs(), _friends(), true)
	assert_array(Array(state.tab_ids())).is_equal(["area", "world", "club:%s" % F111Fixtures.CLUB_SCHOOL,
		"club:%s" % F111Fixtures.CLUB_OWN, "club:%s" % F111Fixtures.CLUB_LOCKED, "friends"])
	assert_str(state.tab("club:%s" % F111Fixtures.CLUB_SCHOOL).title).is_equal("CLB Toán")
	assert_str(state.tab("club:%s" % F111Fixtures.CLUB_LOCKED).status).is_equal(ChatChannelModel.STATUS_READ_ONLY)
	# No tab for the party / match channels (gameplay chat of F119) and none per class or grade (ADR-0095).
	for t: PersistentChannelTab in state.tabs:
		assert_bool(t.channel_type in ["party", "match", "class", "study_group"]).is_false()


func test_area_tab_only_in_a_hub_instance() -> void:
	state.rebuild(_listed(F112Fixtures.channels()), store, _clubs(), _friends(), false)
	assert_object(state.tab("area")).is_null()
	assert_object(state.tab("world")).is_not_null()


func test_area_switches_on_instance_assigned() -> void:
	state.rebuild(_listed(F112Fixtures.channels()), store, _clubs(), _friends(), true)
	assert_str(state.tab("area").channel_id).is_equal(F112Fixtures.AREA)
	var next: Dictionary = F112Fixtures.channels()
	for c: Variant in next["data"] as Array:
		if String((c as Dictionary)["channel_id"]) == F112Fixtures.AREA:
			(c as Dictionary)["channel_id"] = F112Fixtures.AREA_NEXT
	var out: Dictionary = state.rebuild(_listed(next), store, _clubs(), _friends(), true)
	assert_array(Array(out["leave"] as PackedStringArray)).is_equal([F112Fixtures.AREA])
	assert_str(state.tab("area").channel_id).is_equal(F112Fixtures.AREA_NEXT)
	# Leaving the hub (a match) hides the tab and leaves the area channel too.
	out = state.rebuild(_listed(next), store, _clubs(), _friends(), false)
	assert_array(Array(out["leave"] as PackedStringArray)).is_equal([F112Fixtures.AREA_NEXT])
	assert_object(state.tab("area")).is_null()


func test_world_and_area_never_count_unread_only_mentions() -> void:
	state.rebuild(_listed(F112Fixtures.channels()), store, _clubs(), _friends(), true)
	assert_int(state.tab("world").unread).is_equal(0)
	assert_int(state.tab("area").unread).is_equal(0)
	assert_int(state.tab("world").mentions).is_equal(1)
	assert_int(state.tab("club:%s" % F111Fixtures.CLUB_SCHOOL).unread).is_equal(3)


func test_hud_badge_is_the_sum_of_club_and_dm_unread_and_mentions() -> void:
	state.rebuild(_listed(F112Fixtures.channels()), store, _clubs(), _friends(), true)
	# Club CLB Toán 3 + DM CaHeoVui07 1 + DM unknown 2 = 6; World 7 / Area 4 never count.
	assert_int(state.hud_unread()).is_equal(6)
	assert_int(state.hud_mentions()).is_equal(1)
	# `chat.unread.updated` replaces the counts (never adds).
	store.apply_unread({F112Fixtures.TEAM_SCHOOL: 120}, {})
	store.apply_unread({F112Fixtures.TEAM_SCHOOL: 120}, {})
	state.rebuild(state_listed(), store, _clubs(), _friends(), true)
	assert_int(state.hud_unread()).is_equal(123)
	assert_str(ChatHubState.badge_text(state.hud_unread())).is_equal(TranslationServer.translate("ui.chat.unread_many"))


func state_listed() -> Array[ChatChannelModel]:
	var out: Array[ChatChannelModel] = []
	for id: String in store.channel_ids():
		out.append(store.channel(id))
	return out


func test_unknown_dm_only_adds_to_the_friends_badge() -> void:
	state.rebuild(_listed(F112Fixtures.channels()), store, _clubs(), _friends(), true)
	var friends: PersistentChannelTab = state.tab("friends")
	assert_int(friends.unread).is_equal(3)
	assert_bool(state.peers.knows(F112Fixtures.DM_KNOWN)).is_true()
	assert_bool(state.peers.knows(F112Fixtures.DM_UNKNOWN)).is_false()
	assert_int(state.friend_unread(F112Fixtures.FRIEND_DM, store)).is_equal(1)
	# No DM tab until the student opens one; then it is named by the friend's nickname.
	for t: PersistentChannelTab in state.tabs:
		assert_str(t.kind).is_not_equal(PersistentChannelTab.KIND_DM)
	state.open_dm(F112Fixtures.FRIEND_DM)
	state.rebuild(state_listed(), store, _clubs(), _friends(), true)
	assert_str(state.tab("dm:%s" % F112Fixtures.FRIEND_DM).title).is_equal("CaHeoVui07")


func test_no_friends_tab_when_dms_are_off_and_none_listed() -> void:
	state.dm_allowed = false
	var body: Dictionary = F112Fixtures.channels()
	body["data"] = (body["data"] as Array).filter(func(c: Variant) -> bool:
		return String((c as Dictionary)["channel_type"]) != "direct")
	state.rebuild(_listed(body), store, _clubs(), _friends(), true)
	assert_object(state.tab("friends")).is_null()
